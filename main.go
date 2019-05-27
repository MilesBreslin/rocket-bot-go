package main
import (
    "./rocketbot"
    "fmt"
    "time"
)

func main() {
    time.Sleep(time.Second*2)
    for {
        msg := rocketbot.CurrentState.GetMessage()
        fmt.Printf("%s-%s: %s\n",msg.RoomId, msg.UserName, msg.Text)

        if msg.IsNew && msg.IsMention {
            msg.Reply("Hello")
        }
    }
}