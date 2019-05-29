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
        msg, err := rock.GetMessage()
        if err != nil {
            break
        }
        if msg.IsMention && msg.IsNew {
            msg.Reply(fmt.Sprintf("@%s %s", msg.UserName, msg.GetNoMention()))
            msg.React(":grinning:")
        }
    }
}