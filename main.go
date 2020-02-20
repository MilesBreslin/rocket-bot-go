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
    "io/ioutil"
    "gopkg.in/yaml.v2"
    "os"
    "time"
    "math/rand"
    "errors"
    "strconv"
)

type bracket struct {
    Name                string          `yaml:"-"`
    Description         string
    Creator             string
    CreateMessage       string
    SignUpMessages      []string
    Contestants         []string
    Rounds              []bracketRound
    CreatedAt           time.Time
}

type bracketRound []player

type player struct {
    Name                string
    Wins                int
    Losses              int
    Dropped             bool
}

type commandHandlerFunc func(msg rocket.Message, args []string, user string)
type commandHandler struct {
    handler             commandHandlerFunc
    usage               string
    description         string
}

var LONGEST_USAGE int

var commands = map[string]commandHandler {
    "create": commandHandler{
        usage: "<bracket name> [description]",
        description: "Create a new bracket",
        handler: handleCreate,
    },
    "dump": commandHandler{
        usage: "<bracket name>",
        description: "Dump the internal bracket file",
        handler: handleDump,
    },
    "promote": commandHandler{
        usage: "<bracket name>",
        description: "Create a new sign up message",
        handler: handlePromote,
    },
    "close": commandHandler{
        usage: "<bracket name>",
        description: "Close bracket sign ups",
        handler: handleClose, 
    },
    "delete": commandHandler{
        usage: "<bracket name>",
        description: "Delete a bracket",
        handler: handleDelete,
    },
    "list": commandHandler{
        usage: "",
        description: "List all brackets",
        handler: handleList,
    },
    "signup": commandHandler{
        usage: "<bracket name>",
        description: "Signup for a bracket",
        handler: handleSignup,
    },
    "clear": commandHandler{
        usage: "<bracket name>",
        description: "Clear the bracket information",
        handler: handleClear,
    },
    "win-round": commandHandler{
        usage: "<bracket name>",
        description: "Report a won round",
        handler: handleWinRound,
    },
    "loose-round": commandHandler{
        usage: "<bracket name>",
        description: "Report a lost round",
        handler: handleLooseRound,
    },
    "drop": commandHandler{
        usage: "<bracket name>",
        description: "Drop out of a bracket",
        handler: handleDrop,
    },
    "show": commandHandler{
        usage: "<bracket name>",
        description: "Show the state of the bracket",
        handler: handleShow,
    },
    "set-score": commandHandler{
        usage: "<bracket name> <wins-losses>",
        description: "Adjust your score for an inaccuracies",
        handler: handleSetScore,
    },
    "new-round": commandHandler{
        usage: "<bracket name>",
        description: "Force a new round",
        handler: handleNewRound,
    },
}

func init() {
    LONGEST_USAGE = 0
    for n, handler := range commands {
        check_usage := len(handler.usage) + len(n)
        if check_usage > LONGEST_USAGE {
            LONGEST_USAGE = check_usage
        }
    }
}

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
        func() {
            // Wait for a new message to come in
            msg, err := rock.GetNewMessage()

            // If error, quit because that means the connection probably quit
            if err != nil {
                os.Exit(1)
            }

            defer func() {
                if r := recover(); r != nil {
                    msg.Reply(fmt.Sprintf("You made me panic...\n%s",r))
                }
            }()

            // Print the message structure in a user-legible format
            // yml is []byte type, _ means send the returned error to void
            yml, _ := yaml.Marshal(msg)
            fmt.Println(string(yml))

            // If begins with '@Username ' or is in private chat
            if msg.IsAddressedToMe || msg.IsDirect {
                args := strings.Split(msg.GetNotAddressedText(), " ")
                commandName := strings.ToLower(args[0])
                if _, ok := commands[commandName]; !ok || commandName == "usage" || commandName == "help"{
                    reply := "```\n"
                    reply += fmt.Sprintf("Unknown command: %s\n", args[0])
                    reply += fmt.Sprintf("@%s <command> [arguments...] [as @USER]\n", rock.UserName)
                    for command, handler := range commands {
                        usage := fmt.Sprintf("%s %s", command, handler.usage)
                        reply += usage
                        for i := len(usage); i < LONGEST_USAGE + 5 ; i++ {
                            reply += " "
                        }
                        reply += handler.description + "\n"
                    }
                    reply += "```"

                    msg.Reply(reply)
                    return
                }
                user := msg.UserName
                if len(args) > 2 && strings.ToLower(args[len(args)-2]) == "as" {
                    user = strings.ReplaceAll(args[len(args)-1], "@", "")
                    args = args[:len(args)-2]
                }
                commands[commandName].handler(msg, args[1:], user)
            }
        }()
    }
}

func LoadBracket(s string) (bracket, error) {
    var b bracket
    b.Name = strings.ToLower(s)
    bytes, err := ioutil.ReadFile(b.Name + ".yml")
    if err != nil {
        return b, err
    }
    err = yaml.Unmarshal(bytes, &b)
    return b, err
}

func (b *bracket) Write() error {
    bytes, err := yaml.Marshal(b)
    if err != nil {
        return err
    }
    return ioutil.WriteFile(b.Name + ".yml", bytes, 0644)
}

func (b *bracket) Draw() string {
    var drawing string
    round := b.Rounds[len(b.Rounds)-1]
    for _, player := range round {
        drawing += fmt.Sprintf("%s: %d-%d\n", player.Name, player.Wins, player.Losses)
    }
    return drawing
}

func (b *bracket) GetOpponent(user string) (string, error) {
    for index, player := range b.Rounds[len(b.Rounds)-1] {
        if player.Name == user {
            opponentIndex := index - 1 + ((1 - (index % 2)) * 2)
            if opponentIndex > len(b.Rounds[len(b.Rounds)-1])-1 {
                return "", nil
            }
            return b.Rounds[len(b.Rounds)-1][opponentIndex].Name, nil
        }
    }
    return "", errors.New("No such player")
}

func (b *bracket) NewRound() {
    b.Rounds = append(b.Rounds, b.Rounds[len(b.Rounds)-1])

    // Drop any dropped players
    for x := 0 ; x < len(b.Rounds[len(b.Rounds)-1]) ; x++ {
        if b.Rounds[len(b.Rounds)-1][x].Dropped {
            b.Rounds[len(b.Rounds)-1] = append(b.Rounds[len(b.Rounds)-1][:x], b.Rounds[len(b.Rounds)-1][x+1:]...)
        }
    }

    // Sort by player score
    for x := 0 ; x < len(b.Rounds[len(b.Rounds)-1])-1 ; x++ {
        for y := 0 ; y < len(b.Rounds[len(b.Rounds)-1])-1 ; y++ {
            if b.Rounds[len(b.Rounds)-1][y].GetScore() < b.Rounds[len(b.Rounds)-1][y+1].GetScore() {
                b.Rounds[len(b.Rounds)-1][y], b.Rounds[len(b.Rounds)-1][y+1] = b.Rounds[len(b.Rounds)-1][y+1], b.Rounds[len(b.Rounds)-1][y]
            }
        }
    }
}

func (p *player) GetScore() int {
    return p.Wins
}

func (b *bracket) WinRound(user string) error {
    for index, player := range b.Rounds[len(b.Rounds)-1] {
        if player.Name == user {
            b.Rounds[len(b.Rounds)-1][index].Wins += 1
            return nil
        }
    }
    return errors.New("No such player")
}
func (b *bracket) LooseRound(user string) error {
    for index, player := range b.Rounds[len(b.Rounds)-1] {
        if player.Name == user {
            b.Rounds[len(b.Rounds)-1][index].Losses += 1
            return nil
        }
    }
    return errors.New("No such player")
}
func (b *bracket) SetScore(user string, wins int, losses int) error {
    for index, player := range b.Rounds[len(b.Rounds)-1] {
        if player.Name == user {
            b.Rounds[len(b.Rounds)-1][index].Wins = wins
            b.Rounds[len(b.Rounds)-1][index].Losses = losses
            return nil
        }
    }
    return errors.New("No such player")
}

func (b *bracket) RoundIncompletePlayers() []string {
    incompletePlayers := []string{}
    for _, player := range b.Rounds[len(b.Rounds)-1] {
        if len(b.Rounds) == 1 {
            if player.Wins == 0 && player.Losses == 0 {
                incompletePlayers = append(incompletePlayers, player.Name)
            }
        } else {
            for _, prevPlayer := range b.Rounds[len(b.Rounds)-2] {
                if player.Name == prevPlayer.Name {
                    if player.Wins == prevPlayer.Wins && prevPlayer.Losses == prevPlayer.Losses {
                        incompletePlayers = append(incompletePlayers, player.Name)
                    }
                }
            }
        }
    }
    return incompletePlayers
}

func handleCreate(msg rocket.Message, args []string, user string) {
    if len(args) < 1 {
        msg.Reply("Not enough arguments\nSee usage")
        return
    }
    if _, err := LoadBracket(args[0]); err == nil {
        msg.Reply("Bracket already exists")
        return
    }
    reply, err := msg.Reply(fmt.Sprintf("React to this message to sign up for the %s bracket!", args[0]))
    if err != nil {
        return
    }
    b := bracket{
        Name: strings.ToLower(args[0]),
        CreateMessage: msg.Id,
        Creator: msg.UserName,
        CreatedAt: time.Now(),
        SignUpMessages: []string{
            msg.Id,
            reply.Id,
        },
    }
    if len(args) > 1 {
        b.Description = strings.Join(args[1:], " ")
    }


    err = b.Write()
    if err != nil {
        reply.EditText(fmt.Sprintf("Unknown error writing %s\n```\n%s```", args[0], err))
    }
}

func handlePromote(msg rocket.Message, args []string, user string) {
    switch {
    case len(args) < 1:
        msg.Reply("Which brackets do you want to promote")
    default:
        for _, bName := range args {
            b, err := LoadBracket(bName)
            if err != nil {
                msg.Reply(fmt.Sprintf("%s does not exist", bName))
                continue
            }
            reply, err := msg.Reply(fmt.Sprintf("React to this message to sign up for the %s bracket!", args[0]))
            if err != nil {
                break
            }
            b.SignUpMessages = append(b.SignUpMessages, reply.Id, msg.Id)
            err = b.Write()
            if err != nil {
                reply.EditText(fmt.Sprintf("Unknown error writing %s\n```\n%s```", args[0], err))
            }
        }
    }
}

func handleSignup(msg rocket.Message, args []string, user string) {
    if len(args) < 1 {
        msg.Reply("Which brackets do you want to sign up for?")
        return
    }
    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply(args[0] + " does not exist")
    }
    if len(b.Rounds) == 0 {
        b.Contestants = append(b.Contestants, user)
        err = b.Write()
        if err != nil {
            msg.Reply(fmt.Sprintf("Unknown error writing %s\n```\n%s```", args[0], err))
            return
        }
        msg.Reply("@" + user + " signed up for " + args[0])
    } else {
        msg.Reply("Bracket is in progress and not accepting any more sign ups.\nYou must clear the bracket to add more users.")
    }
}

func handleDump(msg rocket.Message, args []string, user string) {
    switch {
    case len(args) < 1:
        msg.Reply("Which brackets do you want to dump?")
    default:
        for _, bName := range args {
            b, err := LoadBracket(bName)
            if err != nil {
                msg.Reply(fmt.Sprintf("%s does not exist", bName))
                continue
            }
            bytes, err := yaml.Marshal(b)
            if err != nil {
                msg.Reply(fmt.Sprintf("%s encountered an unexpected error\n```\n%s\n```", bName, err))
            }
            msg.Reply("```\n" + string(bytes) + "\n```")
        }
    }
}

func handleClose(msg rocket.Message, args []string, user string) {
    if len(args) != 1 {
        msg.Reply("The only argument is a single bracket name")
        return
    }
    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply(args[0] + " does not exist")
        return
    }

    if len(b.Rounds) > 0 {
        msg.Reply(args[0] + " has already been closed")
        return
    }

    // Send Status Message
    statusMsg, err := msg.Reply("Collecting all users")
    if err != nil {
        return
    }
    updateChannel := make(chan string, 0)
    defer close(updateChannel)
    go messageDotsTicker(statusMsg, updateChannel)

    // Collect all users
    for _, signUpMid := range b.SignUpMessages {
        // Needs a timer otherwise it will be blocked
        <- time.After(time.Second)
        signUpMsg, err := msg.RocketCon.RequestMessage(signUpMid)
        if err != nil {
            continue
        }
        for _, rUsers := range signUpMsg.Reactions {
            b.Contestants = append(b.Contestants, rUsers...)
        }
    }

    // Filter to only be unique users
    for x := 0 ; x < len(b.Contestants); x++ {
        for y := x+1 ; y < len(b.Contestants); y++ {
            if b.Contestants[x] == b.Contestants[y] {
                b.Contestants[y] = b.Contestants[len(b.Contestants)-1]
                b.Contestants = b.Contestants[:len(b.Contestants)-1]
                y--
            }
        }
    }

    // Randomize the whole list of contestants
    perm := rand.Perm(len(b.Contestants))
    for i, v := range perm {
        b.Contestants[v], b.Contestants[i] = b.Contestants[i], b.Contestants[v]
    }

    b.Rounds = make([]bracketRound,1)
    for _, contestant := range b.Contestants {
        b.Rounds[0] = append(b.Rounds[0], player{
            Name: contestant,
            Wins: 0,
            Losses: 0,
        })
    }

    err = b.Write()
    if err != nil {
        msg.Reply("Failed to write new bracket")
        return
    }

    updateChannel <- "Revealing players"
    fancyReveal(msg, &b)

}

func handleNewRound(msg rocket.Message, args []string, user string) {
    if len(args) != 1 {
        msg.Reply("Enter only the bracket name")
        return
    }

    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply("Error loading bracket")
        return
    }

    if len(b.Rounds) == 0 {
        msg.Reply("Signups have not closed yet")
        return
    }

    b.NewRound()
    err = b.Write()
    if err != nil {
        msg.Reply("Failed to write new bracket")
        return
    }

    statusMsg, err := msg.Reply("Revealing Players")
    if err != nil {
        return
    }
    updateChannel := make(chan string, 0)
    defer close(updateChannel)
    go messageDotsTicker(statusMsg, updateChannel)

    fancyReveal(msg, &b)
}

func fancyReveal(msg rocket.Message, b *bracket) {
    players := b.Rounds[len(b.Rounds)-1]
    for i := 0 ; i < len(players) ; i++ {
        <- time.After(time.Second)
        if i % 2 == 0 {
            if i == len(players)-1 {
                msg.Reply(fmt.Sprintf("@%s vs the winner of the above match", players[i].Name))
            }
        } else {
            opponent, _ := b.GetOpponent(players[i].Name)
            msg.Reply(fmt.Sprintf("@%s vs @%s", players[i].Name, opponent))
        }
    }
}

func handleWinRound(msg rocket.Message, args []string, user string) {
    if len(args) != 1 {
        msg.Reply("Enter only the bracket name")
        return
    }

    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply("Error loading bracket")
        return
    }

    err = b.WinRound(user)
    if err != nil {
        msg.Reply("User not found in bracket")
        return
    }

    opponent, _ := b.GetOpponent(user)
    if opponent == "" {
        msg.Reply("Unable to find opponent. Please make sure they report their loss")
    } else {
        b.LooseRound(opponent)
        msg.Reply(fmt.Sprintf("%s lost this round", opponent))
    }
    err = b.Write()
    if err != nil {
        msg.Reply("Error writing bracket")
        return
    }
}

func handleLooseRound(msg rocket.Message, args []string, user string) {
    if len(args) != 1 {
        msg.Reply("Enter only the bracket name")
    }

    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply("Error loading bracket")
        return
    }

    err = b.LooseRound(user)
    if err != nil {
        msg.Reply("User not found in bracket")
        return
    }
    opponent, _ := b.GetOpponent(user)
    if opponent == "" {
        msg.Reply("Unable to find opponent. Please make sure they report their win")
    } else {
        b.WinRound(opponent)
        msg.Reply(fmt.Sprintf("%s won this round", opponent))
    }
    err = b.Write()
    if err != nil {
        msg.Reply("Error writing bracket")
        return
    }
}

func handleSetScore(msg rocket.Message, args []string, user string) {
    if len(args) != 2 {
        msg.Reply("Enter only the bracket name and the score (e.g. 2-1)")
        return
    }

    scoreStr := strings.Split(args[1], "-")
    if len(scoreStr) != 2 {
        msg.Reply("Invalid score format (example format: 2-1)")
        return
    }

    wins, _ := strconv.Atoi(scoreStr[0])
    losses, _ := strconv.Atoi(scoreStr[1])

    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply("Error loading bracket")
        return
    }

    b.SetScore(user, wins, losses)

    err = b.Write()
    if err != nil {
        msg.Reply("Error writing bracket")
        return
    }

    msg.React(":thumbsup:")
}

func handleDrop(msg rocket.Message, args []string, user string) {
    msg.Reply("unimplemented")
}

func handleShow(msg rocket.Message, args []string, user string) {
    if len(args) != 1 {
        msg.Reply("Enter only the bracket name")
        return
    }

    b, err := LoadBracket(args[0])
    if err != nil {
        msg.Reply("Error loading bracket")
        return
    }

    text := "```\n" + b.Draw() + "\n```\n"
    incompletePlayers := b.RoundIncompletePlayers()
    for x := 0 ; x < len(incompletePlayers) ; x++ {
        opponentName, _ := b.GetOpponent(incompletePlayers[x])
        if opponentName == "" {
            text += fmt.Sprintf("@%s vs winner of a match and similar rank", incompletePlayers[x])
        } else {
            text += fmt.Sprintf("@%s vs @%s", incompletePlayers[x], opponentName)
        }
        text += "\n"

        for y := x + 1 ; y < len(incompletePlayers) ; y++ {
            if opponentName == incompletePlayers[y] {
                incompletePlayers[y] = incompletePlayers[len(incompletePlayers)-1]
                incompletePlayers = incompletePlayers[:len(incompletePlayers)-1]
                continue
            }
        }
    }
    msg.Reply(text)
}

func handleDelete(msg rocket.Message, args []string, user string) {
    for _, bracket := range args {
        err := os.Remove(bracket + ".yml")
        if err != nil {
            msg.Reply(fmt.Sprintf("%s",err))
            return
        }
        msg.Reply("Deleted " + bracket)
    }
}

func handleList(msg rocket.Message, args []string, user string) {
    files, err := ioutil.ReadDir("./")
    if err != nil {
        msg.Reply(fmt.Sprintf("Error listing files\n```\n%s\n```", err))
    }
    bracketNames := make([]string, 0)
    for _, file := range files {
        fileName := file.Name()
        noSuffix := strings.TrimSuffix(fileName, ".yml")
        if fileName != noSuffix {
            bracketNames = append(bracketNames, noSuffix)
        }
    }
    msg.Reply(fmt.Sprintf("```\n%s\n```",strings.Join(bracketNames, "\n")))
}

func handleClear(msg rocket.Message, args []string, user string) {
    switch {
    case len(args) < 1:
        msg.Reply("Which brackets do you want to clear?")
    default:
        for _, bName := range args {
            b, err := LoadBracket(bName)
            if err != nil {
                msg.Reply(fmt.Sprintf("%s does not exist", bName))
                continue
            }
            b.Rounds = b.Rounds[:0]
            err = b.Write()
            if err != nil {
                msg.Reply("Unable to clear " + bName)
                continue
            }
            msg.Reply(bName + " cleared.")
        }
    }
}

func messageDotsTicker(msg rocket.Message, update chan string) {
    currentText := msg.Text
    for {
        for _, dots := range []int{1,2,3,0} {
            select {
            case val, ok := <- update:
                if !ok {
                    msg.Delete()
                    return
                }
                currentText = val
            case <- time.After(time.Second):
            }
            text := currentText
            for i := 0 ; i < dots ; i++ {
                text += "."
            }
            msg.EditText(text)
        }
    }
}