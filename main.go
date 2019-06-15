package main
import (
    "./rocket"
    "fmt"
    "time"
    "strings"
    "os/exec"
    "os"
    "strconv"
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "errors"
)

// Remind me ___ about ____

type reminder struct {
    quit        chan struct{}
    update      chan rocket.Message
    MsgId       string              `yaml:"msgId"`
    AuxMessages []string            `yaml:"auxMsgs"`
    Expired     bool                `yaml:"Expired"`
    time        time.Time
}

var reminders []*reminder
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
        reminders = make([]*reminder, 0)
    }
}

var rock *rocket.RocketCon

func main() {
    var err error
    rock, err = rocket.NewConnectionConfig("rb.cfg")
    if err != nil {
        panic(err)
    }

    for i := 0; i < len(reminders); i++ {
        if reminders[i].Expired {
            reminders[i] = reminders[len(reminders)-1]
            reminders = reminders[:len(reminders)-1]
            i--
        }
    }

    go manageReminders()
    for _, val := range reminders {
        time.Sleep(time.Second*2)
        msg, err := rock.RequestMessage(val.MsgId)
        if err != nil {
            delete <- *val
        }
        val.quit = make(chan struct{},0)
        val.update = make(chan rocket.Message,0)
        val.time, err = getTimeFromText(msg.GetNotAddressedText())
        if err != nil {
            delete <- *val
        }
        go val.Wait()
    }

    for {
        msg, _ := rock.GetMessage()

        if msg.IsAddressedToMe {
            if msg.IsNew {
                fmt.Printf("%s-%s: %s\n",msg.RoomId, msg.UserName, msg.Text)
            }
            textMsg := msg.GetNotAddressedText()
            if msg.IsNew && strings.HasPrefix(strings.ToLower(textMsg), "help") {
                msg.Reply(fmt.Sprintf("\\@%s me on \\_\\_\\_ about \\_\\_\\_\n\nMost date formats accepted. Uses `date --date \"___\"` to find the time.", rock.UserName))
            } else if msg.IsNew && strings.HasPrefix(strings.ToLower(textMsg), "list") {
                go func() {
                    for i := 0; i < len(reminders); i++ {
                        fmt.Println(i, len(reminders))
                        if !reminders[i].Expired {
                            listmsg, err := rock.RequestMessage(reminders[i].MsgId)
                            if err == nil {
                                if listmsg.RoomId == msg.RoomId {
                                    reply, err := msg.Reply(listmsg.GetQuote())
                                    if err == nil {
                                        reminders[i].update <- reply
                                    }
                                }
                            }
                            time.Sleep(time.Second)
                        }
                    }
                }()
            } else if strings.Contains(strings.ToLower(textMsg),"on ") &&  strings.Contains(strings.ToLower(textMsg)," about ") {
                MsgToReminder(msg)
            }
        }
    }
}

func MsgToReminder(msg rocket.Message) (reminder, error) {
    var rem reminder
    if rem, err := getReminder(msg.Id); err == nil {
        if !msg.IsNew {
            select {
            case <- rem.quit:
                break
            case rem.update <- msg:
                break
            }
        }
        return *rem, nil
    }
    textMsg := msg.GetNotAddressedText()
    if msg.IsAddressedToMe && strings.Contains(strings.ToLower(textMsg),"on ") &&  strings.Contains(strings.ToLower(textMsg)," about ") {
        rem.quit = make(chan struct{},0)
        rem.update = make(chan rocket.Message,0)
        rem.AuxMessages = make([]string,0)
        rem.MsgId = msg.Id
        var err error
        rem.time, err = getTimeFromText(textMsg)
        if err != nil {
            if _, ok := msg.Reactions[":no_entry_sign:"]; !ok {
                msg.React(":no_entry_sign:")
            }
            fmt.Println("Parsing Err")
            close(rem.quit)
            return rem, nil
        } else {
            if _, ok := msg.Reactions[":no_entry_sign:"]; ok {
                msg.React(":no_entry_sign:")
            }
        }
        addReminder(rem)
        msg.Reply("Reminding you and all who react on "+rem.time.Format("1/_2/06 15:04:05"))
        msg.React(":thumbsup:")

        return rem, nil
    }
    return rem, errors.New("Not a valid reminder")
}

func getReminder(mid string) (*reminder, error) {
    for _, val := range reminders  {
        if val.MsgId == mid {
            return val, nil
        }
    }
    return nil, errors.New("No known reminder")
}

func getTimeFromText(s string) (time.Time, error) {
    var t time.Time
    split := strings.Split(strings.ToLower(s), "on ")
    splitt := strings.Split(split[1]," about")
    dateQuery := splitt[0]
    unix, _ := exec.Command("/bin/date","--date",dateQuery,"+%s").Output()
    if len(unix)>2 {
        unixI, err := strconv.ParseInt(string(unix[:len(unix)-1]),10,64)
        if err != nil {
            return t, err
        }

        t = time.Unix(unixI,0)
        fmt.Println(t.Format("1/_2/06 15:04:05"))
        return t, nil
    } else {
        return t, errors.New("Error Parsing time")
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
                    reminders[i].Expired = true
                    fmt.Println("Expired\n\n\n")
                    /*reminders[i] = reminders[len(reminders)-1]
                    reminders = reminders[:len(reminders)-1]
                    i--*/
                }
            }
            writeReminders()
        case r := <- add:
            go r.Wait()
            reminders = append(reminders, &r)
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
    if r.Expired {
        return
    }
    defer func() {
        if rec := recover(); rec != nil {
            fmt.Println("Recovered in f", rec)
            go r.Wait()
        }
    }()
    for {
        tick := time.After(time.Until(r.time))
        fmt.Println("after")
        select {
        case <- r.quit:
            delete <- *r
            return
        case msg := <- r.update:
            var err error
            if msg.Id == "" {
                msg, err = rock.RequestMessage(r.MsgId)
                if err != nil {
                    close(r.quit)
                    break
                }
            }
            if msg.Id == r.MsgId {
                r.time, err = getTimeFromText(msg.GetNotAddressedText())
                if err != nil {
                    close(r.quit)
                    break
                }
            } else {
                r.AuxMessages = append(r.AuxMessages, msg.Id)
                fmt.Println(r.AuxMessages)
                fmt.Println("Appended")
            }
        case <- tick:
            close(r.quit)
            fmt.Println("b4")
            msg, err := rock.RequestMessage(r.MsgId)

            if err != nil {
                break
            }
            aboutMark := strings.Index(strings.ToLower(msg.Text), " about ")
            mentions := fmt.Sprintf("@%s ", msg.UserName)
            fmt.Println("after")
            for _, users := range msg.Reactions {
                for _, user := range users {
                    if user != rock.UserName {
                        mentions = fmt.Sprintf("%s@%s ", mentions, user)
                    }
                }
            }
            for _, msgId := range r.AuxMessages {
                time.Sleep(time.Second)
                auxmsg, err := rock.RequestMessage(msgId)
                if err == nil {
                    for _, users := range auxmsg.Reactions {
                        for _, user := range users {
                            if user != rock.UserName {
                                mentions = fmt.Sprintf("%s@%s ", mentions, user)
                            }
                        }
                    }
                }
            }
            msg.Reply(mentions+msg.Text[aboutMark+7:])
            time.Sleep(time.Second)
            msg.React(":alarm_clock:")
        }
    }
}

func (r *reminder) Destroy() {
    select {
    case <- r.quit:
        break
    default:
        close(r.quit)
    }
}