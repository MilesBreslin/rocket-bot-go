//usr/bin/go run $0 $@ ; exit
// That's a special She-bang for go

// This is a demo rocketbot in golang
// Its purpose is to showcase some features

// Specify we are the main package (the one that contains the main function)
package main

import (
    // Import from the current directory the folder rocket and call the package rocket
    "./rocket"

    "fmt"
    "time"
    "gopkg.in/yaml.v2"
)

func main() {
    // New Connection returning a rocketConnection object
    // rb.cfg is backwards compatible with Kimani's rocket-bot-python
    // Also see NewConnectionPassword and NewConnectionAuthToken
    rock, err := rocket.NewConnectionConfig("rb.cfg")

    rock.UserTemporaryStatus(rocket.STATUS_AWAY)

    // If there was an error connecting, panic
    if err != nil {
        panic(err)
    }

    for {
        // Wait for a new message to come in
        msg, err := rock.GetNewMessage()

        // If error, quit because that means the connection probably quit
        if err != nil {
            break
        }

        // Print the message structure in a user-legible format
        // yml is []byte type, _ means send the returned error to void
        yml, _ := yaml.Marshal(msg)
        fmt.Println(string(yml))

        // If begins with '@Username ' or is in private chat
        if msg.IsAddressedToMe || msg.RoomName == "" {
            // Reply to the message with a formatted string
            reply, err := msg.Reply(fmt.Sprintf("@%s %s %d", msg.UserName, msg.GetNotAddressedText(), 0))

            // If no error replying, take the reply and edit it to count to 10 asynchronously
            if err == nil {
                go func() {
                    msg.SetIsTyping(true)
                    for i := 1; i<=10; i++ {
                        time.Sleep(time.Second)
                        reply.EditText(fmt.Sprintf("@%s %s %d", msg.UserName, msg.GetNotAddressedText(),i))
                    }
                    msg.SetIsTyping(false)
                }()
            }

            // React to the initial message
            msg.React(":grinning:")
        }
    }
}
