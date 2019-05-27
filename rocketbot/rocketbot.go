package rocketbot

import (
    "os"
    "fmt"
    "io/ioutil"
    "time"
    "encoding/json"
    "strings"
    "net/http"

    "github.com/gorilla/websocket"
    "gopkg.in/yaml.v2"
)

type state struct {
    UserId          string          `yaml:"UserId"`
    UserName        string          `yaml:"UserName"`
    AuthToken       string          `yaml:"AuthToken"`
    HostName        string          `yaml:"HostName"`
    HostSSL         bool            `yaml:"HostSSL"`
    HostPort        uint16          `yaml:"HostPort"`
    session         string
    send            chan interface{}
    receive         chan interface{}
    results         map[string] chan map[string] interface{}
    resultsAppend   chan struct {
        string string
        channel chan map[string] interface{}
    }
    resultsDel      chan string
    nextId          chan string
    messages        chan message
}

type message struct {
    IsNew           bool
    IsMention       bool
    IsEdited        bool
    Id              string
    UserName        string
    UserId          string
    RoomName        string
    RoomId          string
    Text            string
    Reactions       map[string] []string
    Attachments     []attachment
    state           state
}

type attachment struct {
    URL             string
}

var CurrentState = state {
    HostPort: 443,
    HostSSL: true,
}

var filename = "rb.cfg"

func init() {
    _, err := os.Stat(filename)
    if err != nil {
        panic(err)
    }

    source, err := ioutil.ReadFile(filename)
    if err != nil {
        panic(err)
    }

    err = yaml.Unmarshal(source, &CurrentState)
    if err != nil {
        panic(err)
    }

    if CurrentState.HostName == "" {
        panic("HostName not set")
    }
    if CurrentState.AuthToken == "" {
        panic("AuthToken not set")
    }


    go CurrentState.run()
}

func (s *state) run() {
    // Init variables
    s.send = make(chan interface{}, 1024)
    s.receive = make(chan interface{}, 1024)
    s.resultsAppend = make(chan struct {
        string string
        channel chan map[string] interface{}
    },0)
    s.resultsDel = make(chan string,1024)
    s.results = make(map[string] chan map[string] interface{})
    s.nextId = make(chan string,0)
    s.messages = make(chan message,1024)

    // Set some websocket tunables
    const socketreadsizelimit = 65536
    const pingtime = 120 * time.Second
    const timeout = 125 * time.Second

    // Define Websocket URL
    var wsURL string
    if s.HostSSL {
        wsURL = fmt.Sprintf("wss://%s:%d/websocket", s.HostName, s.HostPort)
    } else {
        wsURL = fmt.Sprintf("ws://%s:%d/websocket", s.HostName, s.HostPort)
    }

    // Init websocket
    ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
    if err != nil {
        fmt.Println(err)
        return
    }
    defer ws.Close()

    // Configure Websocket using Tunables
    ws.SetReadLimit(socketreadsizelimit)
    ws.SetReadDeadline(time.Now().Add(timeout))
    ws.SetPongHandler(func(string) (error) {
        ws.SetReadDeadline(time.Now().Add(timeout))
        return nil
    })
    tick := time.NewTicker(pingtime)
    defer tick.Stop()

    go func() {
        s.connect()
        s.login()
        s.UserName = s.RequestUserName(s.UserId)
        fmt.Println(s.UserName)
        s.subscribeRooms()
    }()

    // Manage Method/Subscription Ids
    go func() {
        var i uint64
        i=0
        for {
            i++
            s.nextId <- fmt.Sprintf("%d",i)
        }
    }()

    // Manage Results map
    go func() {
        for {
            select {
            case addition := <- s.resultsAppend:
                s.results[addition.string] = addition.channel
            case remove := <- s.resultsDel:
                delete(s.results, remove)
            }
        }
    }()

    // Send Thread
    go func() {
        for {
            msg := <-s.send
            packet, err := json.Marshal(msg)
            err = ws.WriteMessage(websocket.TextMessage, packet)
            if err != nil {
                fmt.Println("Ping Ticker:", err)
                return
            }
        }
    }()

    // Read Thread
    for {
        _, raw, err := ws.ReadMessage()
        ws.SetReadDeadline(time.Now().Add(timeout))

        if err != nil {
            fmt.Printf("error: %v\n", err)
            break
        }

        var pack map[string] interface{}
        err = json.Unmarshal(raw, &pack)
        if err != nil {
            fmt.Printf("error: %v\n", err)
            continue
        }

        if msg, ok := pack["msg"]; ok {
            switch msg {
            case "connected":
                s.session = pack["session"].(string)
            case "result":
                if channel, ok := s.results[pack["id"].(string)]; ok {
                    channel <- pack
                }
                s.resultsDel <- pack["id"].(string)
            case "added":
                switch pack["collection"].(string) {
                case "users":
                    fmt.Println(pack)
                default:
                    fmt.Println(pack)
                }
            case "changed":
                fmt.Println(string(raw))
                obj := pack["fields"].(map[string]interface{})["args"].([]interface{})
                switch pack["collection"].(string) {
                case "stream-notify-user":
                    switch obj[0].(string) {
                    case "inserted":
                        s.subscribeRoom(obj[1].(map[string]interface{})["rid"].(string))
                    }
                case "stream-room-messages":
                    go func() {
                        message := s.handleMessageObject(obj[0].(map[string]interface{}))
                        s.messages <- message
                    }()
                }
            case "ready":
                break
            case "ping":
                pong := map[string] string {
                    "msg": "pong",
                }
                s.send <- pong
            default:
                fmt.Println(string(raw))
            }
        }
    }
}

func (s *state) handleMessageObject(obj map[string] interface{}) message {
    var msg message
    msg.IsNew = true
    _, msg.IsEdited = obj["editedAt"]
    if msg.IsEdited {
        msg.IsNew = false
    }
    msg.Id = obj["_id"].(string)
    msg.Text = obj["msg"].(string)
    msg.RoomId = obj["rid"].(string)
    msg.UserId = obj["u"].(map[string] interface{})["_id"].(string)
    msg.UserName = obj["u"].(map[string] interface{})["username"].(string)

    if len(msg.Text) > len(s.UserName)+2 {
        if string(strings.ToLower(msg.Text)[:len(s.UserName)+2]) == fmt.Sprintf("@%s ", strings.ToLower(s.UserName)) {
            msg.IsMention = true
        }
    }

    if _, ok := obj["unread"]; !ok {
        msg.IsNew = false
    }

    msg.state = *s

    if react, ok := obj["reactions"]; ok {
        msg.IsNew = false
        msg.Reactions = make(map[string][]string)
        for emote, val := range react.(map[string]interface{}) {
            for _, username := range val.(map[string]interface{})["usernames"].([]interface{}) {
                msg.Reactions[emote] = append(msg.Reactions[emote], username.(string))
            }
        }
    }


    // yml, _ := yaml.Marshal(obj)
    // fmt.Println(string(yml))
    // yml, _ = yaml.Marshal(msg)
    // fmt.Println(string(yml))
    return msg
}

func (s *state) generateId() string {
    return <-s.nextId
}

func (s *state) watchResults(str string) chan map[string] interface{} {
    c := make(chan map[string] interface{})
    s.resultsAppend <- struct {
        string string
        channel chan map[string] interface{}
    } {string: str, channel: c}
    return c
}

func (s *state) runMethod(i map[string] interface{}) map[string] interface{} {
    id := s.generateId()
    i["msg"] = "method"
    i["id"] = id
    c := s.watchResults(id)
    defer close(c)
    s.send <- i
    return <- c
}

func (s *state) subscribeRoom(rid string) {
    subscribeRoom := map[string] interface{}{
        "msg": "sub",
        "id": s.generateId(),
        "name": "stream-room-messages",
        "params": []interface{} {
            rid,
            false,
        },
    }
    s.send <- subscribeRoom
}

func (s *state) subscribeRooms() {
    if s.UserId == "" {
        fmt.Println("error: Can't subscribe to rooms if user is not known")
        return
    }
    subscriptionMonitor := map[string] interface{}{
        "msg": "sub",
        "id": s.generateId(),
        "name": "stream-notify-user",
        "params": []interface{} {
            s.UserId+"/subscriptions-changed",
            false,
        },
    }
    s.send <- subscriptionMonitor

    subscriptionsGet := map[string] interface{} {
        "method": "subscriptions/get",
        "params": []map[string] interface{} {
            map[string] interface{} {
                "$date": time.Now().Unix(),
            },
        },
    }
    reply := s.runMethod(subscriptionsGet)

    objects := reply["result"].(map[string] interface{})["update"].([]interface{})

    for index, _ := range objects {
        s.subscribeRoom(objects[index].(map[string]interface{})["rid"].(string))
    }
}

func (s *state) login() {
    login := map[string] interface{} {
        "method": "login",
        "params": []map[string] interface{} {
            map[string] interface{} {
                "resume": s.AuthToken,
            },
        },
    }

    reply := s.runMethod(login)

    s.UserId = reply["result"].(map[string] interface{})["id"].(string)
}

func (s *state) connect() {
    init := map[string] interface{} {
        "msg": "connect",
        "version": "1",
        "support": []string{"1", "pre2", "pre1"},
    }
    s.send <- init
}

func (s *state) RequestUserName(userid string) string {
    res := s.restRequest("/api/v1/users.info?userId="+userid)
    var m map[string] interface{}
    err := json.Unmarshal(res, &m)
    if err != nil {
        fmt.Println(err)
        return ""
    }
    return m["user"].(map[string]interface{})["name"].(string)
}

func (s *state) RequestMessage(mid string) message {
    var msg message
    resp := s.restRequest("/api/v1/chat.getMessage?msgId="+mid)
    var m map[string] interface{}
    err := json.Unmarshal(resp, &m)
    if err != nil {
        fmt.Println(err)
        return msg
    }
    msg = s.handleMessageObject(m["message"].(map[string] interface{}))
    return msg
}

func (s *state) restRequest(str string) []byte{
    // Define Websocket URL
    var httpURL string
    if s.HostSSL {
        httpURL = fmt.Sprintf("https://%s:%d%s", s.HostName, s.HostPort, str)
    } else {
        httpURL = fmt.Sprintf("http://%s:%d%s", s.HostName, s.HostPort, str)
    }
    // Build Request
    client := &http.Client{}
    request, _ := http.NewRequest("GET", httpURL, nil)
    request.Header.Set("X-Auth-Token", s.AuthToken)
    request.Header.Set("X-User-Id", s.UserId)

    // Get Request
    response, _ := client.Do(request)

    // Parse Request
    defer response.Body.Close()
    body, _ := ioutil.ReadAll(response.Body)
    return body
}

func (s *state) SendMessage(rid string, text string) {
    message := map[string] interface{} {
        "method": "sendMessage",
        "params": []map[string] interface{} {
            map[string] interface{} {
                "rid": rid,
                "msg": text,
            },
        },
    }

    s.runMethod(message)
}

func (msg *message) Reply(text string) {
    msg.state.SendMessage(msg.RoomId, text)
}

func (s *state) GetMessage() message {
    return <- s.messages
}