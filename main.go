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

type commandHandlerFunc func(msg rocket.Message, args []string, user string, handler commandHandler)
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
        handler: handleRMWFunc(handleDump, true),
    },
    "promote": commandHandler{
        usage: "<bracket name>",
        description: "Create a new sign up message",
        handler: handleRMWFunc(handlePromote, true),
    },
    "describe": commandHandler{
        usage: "<bracket name>",
        description: "Create a new sign up message",
        handler: handleRMWFunc(handleDescribe, true),
    },
    "close": commandHandler{
        usage: "<bracket name>",
        description: "Close bracket sign ups",
        handler: handleRMWFunc(handleClose, true),
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
        handler: handleRMWFunc(handleSignup, true),
    },
    "clear": commandHandler{
        usage: "<bracket name>",
        description: "Clear the bracket information",
        handler: handleRMWFunc(handleClear, true),
    },
    "win-round": commandHandler{
        usage: "<bracket name>",
        description: "Report a won round",
        handler: handleRMWFunc(handleWinRound, true),
    },
    "loose-round": commandHandler{
        usage: "<bracket name>",
        description: "Report a lost round",
        handler: handleRMWFunc(handleLooseRound, true),
    },
    "drop": commandHandler{
        usage: "<bracket name>",
        description: "Drop out of a bracket",
        handler: handleRMWFunc(handleDrop, true),
    },
    "show": commandHandler{
        usage: "<bracket name>",
        description: "Show the state of the bracket",
        handler: handleRMWFunc(handleShow, true),
    },
    "set-score": commandHandler{
        usage: "<bracket name> <wins-losses>",
        description: "Adjust your score for an inaccuracies",
        handler: handleRMWFunc(handleSetScore, true),
    },
    "new-round": commandHandler{
        usage: "<bracket name>",
        description: "Force a new round",
        handler: handleRMWFunc(handleNewRound, true),
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
                commands[commandName].handler(msg, args[1:], user, commands[commandName])
            }
        }()
    }
}

func LoadBracket(s string) (*bracket, error) {
    var b bracket
    b.Name = strings.ToLower(s)
    bytes, err := ioutil.ReadFile(b.Name + ".yml")
    if err != nil {
        return nil, err
    }
    err = yaml.Unmarshal(bytes, &b)
    return &b, err
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

func (b *bracket) CompileContestants(rock *rocket.RocketCon) {
    // Collect all users
    for _, signUpMid := range b.SignUpMessages {
        // Needs a timer otherwise it will be blocked
        <- time.After(time.Second)
        signUpMsg, err := rock.RequestMessage(signUpMid)
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
}

func (b *bracket) Close(rock *rocket.RocketCon) {
    b.CompileContestants(rock)
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
}

func (b *bracket) IsClosed() bool {
    return ! (len(b.Rounds) == 0)
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
            b.Rounds[len(b.Rounds)-1][x] = b.Rounds[len(b.Rounds)-1][len(b.Rounds[len(b.Rounds)-1])-1]
            b.Rounds[len(b.Rounds)-1] = b.Rounds[len(b.Rounds)-1][:len(b.Rounds[len(b.Rounds)-1])-1]
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

func (b *bracket) Drop(user string) error {
    for index, player := range b.Rounds[len(b.Rounds)-1] {
        if player.Name == user {
            b.Rounds[len(b.Rounds)-1][index].Dropped = true
            return nil
        }
    }
    return errors.New("No such player")
}

func (b *bracket) RoundIncompletePlayers() []string {
    incompletePlayers := []string{}
    if ! b.IsClosed(){
        return incompletePlayers
    }
    for _, player := range b.Rounds[len(b.Rounds)-1] {
        if len(b.Rounds) == 1 {
            if player.Wins == 0 && player.Losses == 0 {
                incompletePlayers = append(incompletePlayers, player.Name)
            }
        } else {
            for _, prevPlayer := range b.Rounds[len(b.Rounds)-2] {
                if player.Name == prevPlayer.Name {
                    if player.Wins == prevPlayer.Wins && player.Losses == prevPlayer.Losses {
                        incompletePlayers = append(incompletePlayers, player.Name)
                    }
                }
            }
        }
    }
    return incompletePlayers
}

func handleRMWFunc(f func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error), bracketRequired bool) (func(msg rocket.Message, args []string, user string, handler commandHandler)) {
    return func(msg rocket.Message, args []string, user string, handler commandHandler) {
        if len(args) < 1 {
            msg.Reply("Not enough arguments\nSee usage")
            return
        }
        b, err := LoadBracket(args[0])
        if err != nil {
            if bracketRequired {
                msg.Reply("Bracket not found")
                return
            }
        }
        reply, err := f(msg, args[1:], user, handler, b)
        if err != nil {
            return
        }
        if b.IsClosed() && len(b.RoundIncompletePlayers()) == 0 {
            reply, err = handleNewRound(msg, args[1:], user, handler, b)
        }
        if err != nil {
            return
        }
        err = b.Write()
        if err != nil {
            if reply == nil {
                msg.Reply("Error writing bracket " + b.Name)
            } else {
                reply.EditText("Error writing bracket " + b.Name)
            }
        }
    }
}

func handleCreate(msg rocket.Message, args []string, user string, handler commandHandler) {
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

func handlePromote(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    reply, err := msg.Reply(fmt.Sprintf("React to this message to sign up for the %s bracket!\n```\n%s\n```", b.Name, b.Description))
    if err != nil {
        return nil, err
    }
    b.SignUpMessages = append(b.SignUpMessages, reply.Id, msg.Id)
    return &reply, nil
}

func handleSignup(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    if !b.IsClosed() {
        b.Contestants = append(b.Contestants, user)
        reply, err := msg.Reply("@" + user + " signed up for " + b.Name)
        return &reply, err
    } else {
        msg.Reply("Bracket is in progress and not accepting any more sign ups.\nYou must clear the bracket to add more users.")
        return nil, errors.New("Do not write")
    }
}

func handleDump(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    noWrite := errors.New("Do not write")
    bytes, err := yaml.Marshal(b)
    if err != nil {
        msg.Reply(fmt.Sprintf("%s encountered an unexpected error\n```\n%s\n```", b.Name, err))
        return nil, noWrite
    }
    msg.Reply("```\n" + string(bytes) + "\n```")
    return nil, nil
}

func handleDescribe(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    b.Description = strings.Join(args, " ")
    msg.React(":thumbsup:")
    return nil, nil
}

func handleClose(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    if b.IsClosed() {
        msg.Reply(args[0] + " has already been closed")
        return nil, errors.New("Do not write)")
    }

    // Send Status Message
    statusMsg, err := msg.Reply("Collecting all users")
    if err != nil {
        return nil, err
    }
    updateChannel := make(chan string, 0)
    defer close(updateChannel)
    go messageDotsTicker(statusMsg, updateChannel)

    // Close
    b.Close(msg.RocketCon)

    updateChannel <- "Revealing players"
    fancyReveal(msg, b)
    return nil, nil
}

func handleNewRound(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    if !b.IsClosed() {
        msg.Reply("Signups have not closed yet")
        return nil, errors.New("Do not write")
    }

    b.NewRound()

    statusMsg, err := msg.Reply("Revealing Players")
    if err != nil {
        return nil, err
    }
    updateChannel := make(chan string, 0)
    defer close(updateChannel)
    go messageDotsTicker(statusMsg, updateChannel)

    fancyReveal(msg, b)
    return nil, nil
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

func handleWinRound(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    var reply rocket.Message
    err := b.WinRound(user)
    if err != nil {
        reply, _ = msg.Reply("User not found in bracket")
        return &reply, err
    }

    opponent, _ := b.GetOpponent(user)
    if opponent == "" {
        reply, err = msg.Reply("Unable to find opponent. Please make sure they report their loss")
    } else {
        b.LooseRound(opponent)
        reply, err = msg.Reply(fmt.Sprintf("%s lost this round", opponent))
    }
    return &reply, err
}

func handleLooseRound(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    var reply rocket.Message
    err := b.LooseRound(user)
    if err != nil {
        reply, _ = msg.Reply("User not found in bracket")
        return &reply, err
    }
    opponent, _ := b.GetOpponent(user)
    if opponent == "" {
        reply, err = msg.Reply("Unable to find opponent. Please make sure they report their win")
    } else {
        b.WinRound(opponent)
        reply, err = msg.Reply(fmt.Sprintf("@%s won this round", opponent))
    }
    return &reply, err
}

func handleSetScore(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    if len(args) != 1 {
        reply, err := msg.Reply("Enter only the bracket name and the score (e.g. 2-1)")
        return &reply, err
    }

    scoreStr := strings.Split(args[0], "-")
    if len(scoreStr) != 2 {
        reply, err := msg.Reply("Invalid score format (example format: 2-1)")
        return &reply, err
    }

    wins, _ := strconv.Atoi(scoreStr[0])
    losses, _ := strconv.Atoi(scoreStr[1])

    b.SetScore(user, wins, losses)

    msg.React(":thumbsup:")
    return nil, nil
}

func handleDrop(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    err := b.Drop(user)
    if err != nil {
        reply, _ := msg.Reply(fmt.Sprintf("%s", err))
        return &reply, errors.New("Do not write")
    }
    reply, err := handleLooseRound(msg, args, user, handler, b)
    reply.EditText(fmt.Sprintf("%s\n%s has dropped this round.", reply.Text, user))
    return reply, nil
}

func handleShow(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    if b.IsClosed() {
        text := fmt.Sprintf("Bracket Name: %s\n", b.Name)
        text += fmt.Sprintf("Description: %s\n", b.Description)
        text += fmt.Sprintf("Round: %d\n", len(b.Rounds))
        text += "```\n" + b.Draw() + "\n```\n"
        text += "Incomplete matches:\n"
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
    } else {
        statusMsg, err := msg.Reply("Collecting all users")
        if err != nil {
            return nil, err
        }
        updateChannel := make(chan string, 0)
        defer close(updateChannel)
        go messageDotsTicker(statusMsg, updateChannel)

        b.CompileContestants(msg.RocketCon)
        text := fmt.Sprintf("%s is currently open.\n%s\n", b.Name, b.Description)
        text += "```\n"
        text += strings.Join(b.Contestants, "\n")
        text += "\n```"
        msg.Reply(text)
    }
    return nil, errors.New("Do not write")
}

func handleDelete(msg rocket.Message, args []string, user string, handler commandHandler) {
    for _, bracket := range args {
        err := os.Remove(bracket + ".yml")
        if err != nil {
            msg.Reply(fmt.Sprintf("%s",err))
            return
        }
        msg.Reply("Deleted " + bracket)
    }
}

func handleList(msg rocket.Message, args []string, user string, handler commandHandler) {
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

func handleClear(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    b.Rounds = b.Rounds[:0]
    reply, err := msg.Reply(b.Name + " cleared.")
    return &reply, err
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