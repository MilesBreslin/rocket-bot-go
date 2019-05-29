package main
import (
    "./rocketbot"
    "fmt"
)

func main() {
    rock, err := rocketbot.NewConnectionConfig("rb.cfg")
    if err != nil {
        panic(err)
    }

    
    for {
        msg, err := rock.GetMessage()
        if err != nil {
            break
        }
        if msg.IsMention && msg.IsNew {
            msg.Reply(fmt.Sprintf("@%s %s", msg.UserName, msg.GetNoMention()))
        }
    }
}