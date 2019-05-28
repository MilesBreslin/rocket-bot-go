package main
import (
    "./rocketbot"
    "fmt"
    "time"
    "strings"
    "os/exec"
    "os"
    "strconv"
    "io/ioutil"
    "gopkg.in/yaml.v2"
)

// Remind me ___ about ____

type reminder struct {
    quit        chan struct{}
    MsgId       string              `yaml:"msgId"`
    Time        time.Time           `yaml:"time"`
}

var reminders []reminder
var filename = "reminder.yml"
var add = make(chan reminder, 128)
var delete = make(chan reminder, 128)

func init() {
    _, err := os.Stat(filename)
    if err == nil {
        source, err := ioutil.ReadFile(filename)
        if err != nil {
            panic(err)
        }

        err = yaml.Unmarshal(source, &reminders)
        if err != nil {
            panic(err)
        }
    } else {
        reminders = make([]reminder, 0)
    }
}

func main() {
    go manageReminders()
    for _, val := range reminders {
        val.quit = make(chan struct{},0)
        go val.Wait()
    }
    for {
        msg := rocketbot.CurrentState.GetMessage()

        if msg.IsNew && msg.IsMention {
            fmt.Printf("%s-%s: %s\n",msg.RoomId, msg.UserName, msg.Text)
            textMsg := msg.Text[len(rocketbot.CurrentState.UserName)+2:]
            if strings.Contains(strings.ToLower(textMsg)," on ") &&  strings.Contains(strings.ToLower(textMsg)," about ") {
                split := strings.Split(strings.ToLower(textMsg), " on ")
                splitt := strings.Split(split[1]," about")
                dateQuery := splitt[0]
                unix, _ := exec.Command("/bin/date","--date",dateQuery,"+%s").Output()
                var rem reminder
                rem.quit = make(chan struct{},0)
                rem.MsgId = msg.Id
                unixI, _:= strconv.ParseInt(string(unix[:len(unix)-1]),10,64)
                fmt.Printf("s%ss\ni%di\n",unix,unixI)
                rem.Time = time.Unix(unixI,0)
                addReminder(rem)

                msg.Reply("Reminding you and all who react on "+rem.Time.Format("1/_2/06 15:04:05"))
            }
            
        }
    }
}

func addReminder(r reminder) {
    add <- r
}

func manageReminders() {
    for {
        select {
        case r := <- delete:
            for i := 0; i<len(reminders); i++ {
                if reminders[i].MsgId == r.MsgId {
                    reminders[i] = reminders[len(reminders)-1]
                    reminders = reminders[:len(reminders)-1]
                    i--
                }
            }
            writeReminders()
        case r := <- add:
            go r.Wait()
            reminders = append(reminders, r)
            writeReminders()
        }
    }
}

func writeReminders() {
    yml, err := yaml.Marshal(&reminders)
    if err != nil {
        return
    }
    ioutil.WriteFile(filename, yml, 0644)
    fmt.Println("Reminders written")
}

func (r *reminder) Wait() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("Recovered in f", r)
        }
    }()
    rocketbot.CurrentState.WaitUntilReady()
    tick := time.After(time.Until(r.Time))
    select {
    case <- r.quit:
        break
    case <- tick:
        close(r.quit)
        msg := rocketbot.CurrentState.RequestMessage(r.MsgId)
        about := strings.Split(msg.Text, " about ")
        mentions := ""
        for _, users := range msg.Reactions {
            for _, user := range users {
                mentions = fmt.Sprintf("%s@%s ", mentions, user)
            }
        }
        msg.Reply(mentions+about[1])
    }
    delete <- *r
}