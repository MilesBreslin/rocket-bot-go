package main
import (
    "fmt"
    "time"
    "log"

    "encoding/json"
    "github.com/gorilla/websocket"
)

func main() {
    const socketreadsizelimit = 1024
    const pingtime = 120 * time.Second
    const timeout = 125 * time.Second

    // Init websocket
    ws, _, err := websocket.DefaultDialer.Dial("wss://rocket.cat.pdx.edu/websocket", nil)
    if err != nil {
        fmt.Println(err)
        return
    }

    // Configure Websocket
    ws.SetReadLimit(socketreadsizelimit)
    ws.SetReadDeadline(time.Now().Add(timeout))
    ws.SetPongHandler(func(string) (error) {
        ws.SetReadDeadline(time.Now().Add(timeout))
        return nil
    })
    tick := time.NewTicker(pingtime)
    defer tick.Stop()

    c := make(chan interface{}, 2)

    go func() {
        init := struct {
            Message     string      `json:"msg"`
            Version     string      `json:"version"`
            Support     []string    `json:"support"`
        }{
            Message: "connect",
            Version: "1",
            Support: make([]string,3),
        }
        init.Support[0] = "1"
        init.Support[1] = "pre2"
        init.Support[2] = "pre1"
        c <- init

        login := struct {
            Message     string      `json:"msg"`
            Method      string      `json:"method"`
            Login       string      `json:"login"`
            Id          string      `json:"id"`
            Params      []struct
            {
                Resume  string      `json:"resume"`
            }                       `json:"params"`
        }{
            Message: "method",
            Method: "login",
            Id: "1",
        }
        login.Params = make([]struct { Resume string `json:"resume"` },1)
        login.Params[0].Resume = "Auth Token"

        c <- login
    }()

    go func() {
        for {
            msg := <-c
            packet, err := json.Marshal(msg)
            err = ws.WriteMessage(websocket.TextMessage, packet)
            if err != nil {
                log.Println("Ping Ticker:", err)
                return
            }
        }
    }()

    for {
        _, msg, err := ws.ReadMessage()
        ws.SetReadDeadline(time.Now().Add(timeout))

        if err != nil {
            log.Printf("error: %v", err)
            break
        }

        fmt.Println(string(msg))
    }
}