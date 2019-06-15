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
    "io/ioutil"
    "os"
    "strings"
    "gopkg.in/yaml.v2"
)

type entry struct {
    Statement           string
    Reply               string
    UserName            string
    RoomId              string
    MsgId               string
}

var entries []entry
var filename = "iSay.cfg"

func init() {
    entries = make([]entry, 0)
    _, err := os.Stat(filename)
    if err != nil {
        return
    }

    source, err := ioutil.ReadFile(filename)
    if err != nil {
        return
    }

    err = yaml.Unmarshal(source, &entries)
    if err != nil {
        return
    }
}

func main() {
    rock, err := rocket.NewConnectionConfig("rb.cfg")
    if err != nil {
        panic(err)
    }

    for {
        // Wait for a new message to come in
        msg, err := rock.GetNewMessage()
        if err != nil {
            break
        }

        var statement string
        var reply string
        _, err = fmt.Sscanf(msg.GetNotAddressedText(), "when i say %s you say %s", &statement, &reply)
        if err == nil {
            if statement[len(statement)-1:len(statement)] == "," {
                statement=statement[:len(statement)-1]
            }
            if reply[len(reply)-1:len(reply)] == "." {
                reply=reply[:len(reply)-1]
            }
            entries = append(entries, entry{
                Statement: statement,
                Reply: reply,
                UserName: msg.UserName,
                RoomId: msg.RoomId,
                MsgId: msg.Id,
            })
            writeDatabase()
        } else {
            fmt.Println(err)
            if msg.IsAddressedToMe {
                success := false
                parsableText := strings.ToLower(msg.GetNotAddressedText())
                if postfix := getTextAfter(parsableText, "delete "); postfix != "" {
                    if postfix := getTextAfter(postfix, " i say "); postfix != "" {
                        for i := 0; i<len(entries); i++ {
                            if strings.ToLower(entries[i].Statement) == postfix {
                                success = true
                                rock.React(entries[i].MsgId, ":skull_crossbones:")
                                msg.Reply(fmt.Sprintf("%s\n--Delted--",entries[i].toText()))
                                entries[i] = entries[len(entries)-1]
                                entries = entries[:len(entries)-1]
                                break
                                i--
                            }
                        }
                    }
                    if postfix := getTextAfter(postfix, " you say "); postfix != "" {
                        for i := 0; i<len(entries); i++ {
                            if strings.ToLower(entries[i].Reply) == postfix {
                                success = true
                                rock.React(entries[i].MsgId, ":skull_crossbones:")
                                msg.Reply(fmt.Sprintf("%s\n--Delted--",entries[i].toText()))
                                entries[i] = entries[len(entries)]
                                entries = entries[:len(entries)-1]
                                i--
                            }
                        }
                    }
                    if success {
                        writeDatabase()
                    } else {
                        msg.Reply("I didn't understand you.\nDelete when {i,you} say \\_\\_\\_")
                    }
                } else {
                    msg.Reply("Speak properly!\nWhen I say \\_\\_\\_, you say \\_\\_\\_.\nYou can also say \"@"+rock.UserName+" delete when {I,you} say \\_\\_\\_")
                }
            }
        }
    }
}

func writeDatabase() {
    yml, err := yaml.Marshal(entries)
    if err != nil {
        return
    }
    fmt.Println(string(yml))

    err = ioutil.WriteFile(filename, yml, 0644)
    if err != nil {
        return
    }
}

func getTextAfter(initial string, match string) (string) {
    if index := strings.Index(initial, match); index != -1 {
        return initial[index+len(match):]
    } else {
        return ""
    }
}

func (e *entry) toText() (string) {
    yml, _ := yaml.Marshal(e)
    return string(yml)
}