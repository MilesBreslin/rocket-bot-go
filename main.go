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
    "strings"
    "math/rand"
    "os"
)

func main() {
    // New Connection returning a rocketConnection object
    // rb.cfg is backwards compatible with Kimani's rocket-bot-python
    // Also see NewConnectionPassword and NewConnectionAuthToken
    rock, err := rocket.NewConnectionConfig("rb.cfg")

    // If there was an error connecting, panic
    if err != nil {
        panic(err)
    }

    emojis, err := rock.ListCustomEmojis()
    if err != nil {
        panic(err)
    }

    for i := 0; i < len(emojis); i++ {
        if ! strings.Contains(emojis[i], "parrot") {
            fmt.Println("removing", emojis[i])
            emojis[i] = emojis[len(emojis)-1]
            emojis = emojis[:len(emojis)-1]
            i--
        }
    }
    fmt.Println(emojis)

    for {
        // Wait for a new message to come in
        msg, err := rock.GetMessage()

        // If error, quit because that means the connection probably quit
        if err != nil {
            break
        }

        for _, username := range os.Args[1:] {
            if strings.HasPrefix(strings.ToLower(msg.Text), fmt.Sprintf("@%s",username)) || msg.UserName == username {
                if len(msg.Reactions) == 0 {
                    msg.React(emojis[rand.Intn(len(emojis))])
                }
            }
        }
    }
}
