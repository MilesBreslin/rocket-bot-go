package main
import (
    "./rocket"
    "fmt"
)

func main() {
    rock, err := rocket.NewConnectionConfig("rb.cfg")
    if err != nil {
        panic(err)
    }


    for {
        msg, err := rock.GetIncommingMessage()
        if err != nil {
            break
        }
        if msg.IsMention {
            msg.Reply(fmt.Sprintf("@%s %s", msg.UserName, msg.GetNoMention()))
            msg.React(":grinning:")
        }
    }
}