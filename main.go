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
    "math/rand"
    "strings"
)

func main() {
    rand.Seed(time.Now().Unix())
    rock, err := rocket.NewConnectionConfig("rb.cfg")

    // If there was an error connecting, panic
    if err != nil {
        panic(err)
    }
    customEmojis, _ := rock.ListCustomEmojis()
    emojis := append(rocket.BUILTIN_EMOJIS, customEmojis...)
    totalemojis := len(emojis)

    for {
        // Wait for a new message to come in
        msg, err := rock.GetMessage()

        // If error, quit because that means the connection probably quit
        if err != nil {
            break
        }

        // If begins with '@Username ' or is in private chat
        if msg.IsNew && msg.IsAddressedToMe && strings.Contains(strings.ToLower(msg.GetNotAddressedText()), " _emoji_ ") {
            fmt.Println(msg.UserName)
            go func() {
                complete := make([]int,0)
                for len(complete)<len(emojis)/4 { 
                    if len(complete) < 10 {
                        time.Sleep(time.Millisecond*250)
                    } else {
                        time.Sleep(time.Second)
                    }
                    i := rand.Intn(totalemojis)
                    for invalid(i, complete) {
                        i = rand.Intn(totalemojis)
                    }
                    err := msg.React(emojis[i])
                    if err == nil {
                        complete = append(complete, i)
                        fmt.Println(len(complete))
                    }
                }
            }()
        }
    }
}

func invalid(i int, arr []int) bool {
    for _, val := range arr {
        if i == val {
            return true
        }
    }
    return false
}
>>>>>>> Emoji bot
