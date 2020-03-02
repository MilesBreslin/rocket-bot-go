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
var ERR_DO_NOT_WRITE = errors.New("Do not write")
var ERR_SIGNUPS_NOT_CLOSED = errors.New("Sign ups are not closed")
var ERR_SIGNUPS_CLOSED = errors.New("Sign ups are closed")

var commands = map[string]commandHandler {
    "create": commandHandler{
        usage: "<bracket name> [description]",
        description: "Create a new bracket",
        handler: func(msg rocket.Message, args []string, user string, handler commandHandler) {
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
        },
    },
    "dump": commandHandler{
        usage: "<bracket name>",
        description: "Dump the internal bracket file",
        handler: quarantineCommand(handleROFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) {
            if ! msg.IsDirect && ! strings.Contains(msg.RoomName, "bot") {
                msg.Reply("To reduce spam, dump command is only available in a *direct message or a bot room*.")
                return
            }
            bytes, err := yaml.Marshal(b)
            if err != nil {
                msg.Reply(fmt.Sprintf("%s encountered an unexpected error\n```\n%s\n```", b.Name, err))
                return
            }
            msg.Reply("```\n" + string(bytes) + "\n```")
        })),
    },
    "promote": commandHandler{
        usage: "<bracket name>",
        description: "Create a new sign up message",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            reply, err := msg.Reply(fmt.Sprintf("React to this message to sign up for the %s bracket!\n```\n%s\n```", b.Name, b.Description))
            if err != nil {
                return nil, err
            }
            b.SignUpMessages = append(b.SignUpMessages, reply.Id, msg.Id)
            return &reply, nil
        }),
    },
    "describe": commandHandler{
        usage: "<bracket name>",
        description: "Set the description of the bracket",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            if len(args) == 0 {
                msg.Reply(fmt.Sprintf("Cannot set the description of %s to nothing.", b.Name))
                return nil, ERR_DO_NOT_WRITE
            }
            b.Description = strings.Join(args, " ")
            msg.React(":thumbsup:")
            return nil, nil
        }),
    },
    "close": commandHandler{
        usage: "<bracket name>",
        description: "Close bracket sign ups",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            if b.IsClosed() {
                return nil, ERR_SIGNUPS_CLOSED
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
        }),
    },
    "delete": commandHandler{
        usage: "<bracket name>",
        description: "Delete a bracket",
        handler: func(msg rocket.Message, args []string, user string, handler commandHandler) {
            for _, bracket := range args {
                err := os.Remove(bracket + ".yml")
                if err != nil {
                    msg.Reply(fmt.Sprintf("%s",err))
                    return
                }
                msg.Reply("Deleted " + bracket)
            }
        },
    },
    "list": commandHandler{
        usage: "",
        description: "List all brackets",
        handler: func(msg rocket.Message, args []string, user string, handler commandHandler) {
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
        },
    },
    "signup": commandHandler{
        usage: "<bracket name>",
        description: "Signup for a bracket",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            if !b.IsClosed() {
                b.Contestants = append(b.Contestants, user)
                reply, err := msg.Reply("@" + user + " signed up for " + b.Name)
                return &reply, err
            } else {
                return nil, ERR_SIGNUPS_CLOSED
            }
        }),
    },
    "clear": commandHandler{
        usage: "<bracket name>",
        description: "Clear the bracket information",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            b.Rounds = b.Rounds[:0]
            reply, _ := msg.Reply(b.Name + " cleared.")
            return &reply, nil
        }),
    },
    "win-round": commandHandler{
        usage: "<bracket name>",
        description: "Report a won round",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            var reply rocket.Message
            err := b.WinRound(user)
            if err != nil {
                return nil, err
            }

            opponent, _ := b.GetOpponent(user)
            if opponent == "" || strings.Contains(opponent, " ") {
                reply, err = msg.Reply("Unable to find opponent. Please make sure they report their loss")
            } else {
                b.LooseRound(opponent)
                reply, err = msg.Reply(fmt.Sprintf("@%s lost this round", opponent))
            }
            return &reply, err
        }),
    },
    "lose-round": commandHandler{
        usage: "<bracket name>",
        description: "Report a lost round",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            var reply rocket.Message
            err := b.LooseRound(user)
            if err != nil {
                return nil, err
            }
            opponent, _ := b.GetOpponent(user)
            if opponent == "" || strings.Contains(opponent, " ") {
                reply, err = msg.Reply("Unable to find opponent. Please make sure they report their win")
            } else {
                b.WinRound(opponent)
                reply, err = msg.Reply(fmt.Sprintf("@%s won this round", opponent))
            }
            return &reply, err
        }),
    },
    "drop": commandHandler{
        usage: "<bracket name>",
        description: "Drop out of a bracket",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
            err := b.Drop(user)
            if err != nil {
                return nil, ERR_DO_NOT_WRITE
            }
            if ! b.HasPlayed(user) {
                b.LooseRound(user)
                opponent, _ := b.GetOpponent(user)
                if strings.Contains(opponent, " ") {
                    b.WinRound(opponent)
                    msg.Reply(opponent + " has won this round by default.")
                }
            }
            reply, _ := msg.Reply(fmt.Sprintf("%s has dropped this round.", user))
            return &reply, nil
        }),
    },
    "show": commandHandler{
        usage: "<bracket name>",
        description: "Show the state of the bracket",
        handler: handleROFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) {
            shouldSpam := false
            if len(args) > 0 && strings.HasSuffix(args[0], "spam") {
                shouldSpam = true
            }
            if b.IsClosed() {
                text := fmt.Sprintf("Bracket Name: %s\n", b.Name)
                text += fmt.Sprintf("Description: %s\n", b.Description)
                text += fmt.Sprintf("Round: %d\n", len(b.Rounds))
                if msg.IsDirect {
                    text += "```\n" + b.Draw() + "\n```\n"
                }
                namePrefix := ""
                if msg.IsDirect || shouldSpam {
                    namePrefix = "@"
                }
                text += "Incomplete matches:\n"
                incompletePlayers := b.RoundIncompletePlayers()
                for x := 0 ; x < len(incompletePlayers) ; x++ {
                    opponentName, _ := b.GetOpponent(incompletePlayers[x])
                    if strings.Contains(opponentName, " ") {
                        text += fmt.Sprintf("%s vs %s", namePrefix + incompletePlayers[x], opponentName)
                    } else {
                        text += fmt.Sprintf("%s vs %s", namePrefix + incompletePlayers[x], namePrefix + opponentName)
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
                    return
                }
                updateChannel := make(chan string, 0)
                defer close(updateChannel)
                go messageDotsTicker(statusMsg, updateChannel)

                b.CompileContestants(msg.RocketCon)
                msg.Reply(fmt.Sprintf("%s signups are open.\n**Description:** %s\n**Contestants:** %s", b.Name, b.Description, strings.Join(b.Contestants, ", ")))
            }
        }),
    },
    "set-score": commandHandler{
        usage: "<bracket name> <wins-losses>",
        description: "Adjust your score for an inaccuracies",
        handler: handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
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
        }),
    },
    "new-round": commandHandler{
        usage: "<bracket name>",
        description: "Force a new round",
        handler: handleRMWFunc(handleNewRound),
    },
    "source-code": commandHandler{
        usage: "",
        description: "Give a link to the source code",
        handler: func(msg rocket.Message, args []string, user string, handler commandHandler) {
            msg.Reply("https://github.com/MilesBreslin/rocket-bot-go/tree/bracket_bot")
        },
    },
}

//////////////////
// Program Init //
//////////////////

func init() {
    // Compute the longest description
    // Allows the usage info to all be aligned automatically
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
                    var reply string
                    if msg.IsDirect || strings.Contains(msg.RoomName, "bot") {
                        reply = fmt.Sprintf("Unknown command %s. To reduce spam, please view the `help` command in a *direct message or a bot channel*.", commandName)
                    } else {
                        reply = "```\n"
                        reply += fmt.Sprintf("Unknown command: %s\n", commandName)
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
                    }

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

/////////////////////
// Handler Helpers //
/////////////////////

func quarantineCommand(f func(msg rocket.Message, args []string, user string, handler commandHandler)) func(msg rocket.Message, args []string, user string, handler commandHandler) {
    return func(msg rocket.Message, args []string, user string, handler commandHandler) {
        if msg.IsDirect || strings.Contains(msg.RoomName, "bot") {
            f(msg, args, user, handler)
        } else {
            if strings.HasSuffix(args[0], "spam") {
                f(msg, args[1:], user, handler)
            } else {
                msg.Reply(fmt.Sprintf("This command is quarantined from general channels to reduce spam. Please use it in a bot room or a direct message."))
            }
        }
    }
}

func handleROFunc(f func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket)) (func(msg rocket.Message, args []string, user string, handler commandHandler)) {
    return handleRMWFunc(func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
        f(msg, args, user, handler, b)
        return nil, ERR_DO_NOT_WRITE
    })
}

func handleRMWFunc(f func(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error)) (func(msg rocket.Message, args []string, user string, handler commandHandler)) {
    return func(msg rocket.Message, args []string, user string, handler commandHandler) {
        if len(args) < 1 {
            msg.Reply("Not enough arguments\nSee usage")
            return
        }
        b, err := LoadBracket(args[0])
        if err != nil {
            msg.Reply("Bracket not found")
            return
        }
        reply, err := f(msg, args[1:], user, handler, b)
        if err != nil && err != ERR_DO_NOT_WRITE {
            msg.Reply(fmt.Sprintf("%s", err))
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

func fancyReveal(msg rocket.Message, b *bracket) {
    players := b.Rounds[len(b.Rounds)-1]
    for i := 0 ; i < len(players) ; i++ {
        <- time.After(5 * time.Second)
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

func messageDotsTicker(msg rocket.Message, update chan string) {
    defer msg.SetIsTyping(false)
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
            case <- time.After(2 * time.Second):
            }
            msg.SetIsTyping(true)
            if ! msg.IsDirect && ! strings.Contains(msg.RoomName, "bot") {
                text := currentText
                for i := 0 ; i < dots ; i++ {
                    text += "."
                }
                msg.EditText(text)
            }
        }
    }
}

func handleNewRound(msg rocket.Message, args []string, user string, handler commandHandler, b *bracket) (*rocket.Message, error) {
    if !b.IsClosed() {
        return nil, ERR_SIGNUPS_NOT_CLOSED
    }

    b.NewRound()

    statusMsg, _ := msg.Reply("Revealing Players")
    updateChannel := make(chan string, 0)
    defer close(updateChannel)
    go messageDotsTicker(statusMsg, updateChannel)

    fancyReveal(msg, b)
    return nil, nil
}

///////////////////////
// Bracket Functions //
///////////////////////

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

func (b *bracket) HasPlayed(user string) (bool) {
    isIncomplete := false
    incompletePlayers := b.RoundIncompletePlayers()
    for _, i := range incompletePlayers {
        isIncomplete = isIncomplete || (i == user)
    }
    return !isIncomplete
}

func (b *bracket) GetOpponent(user string) (string, error) {
    r := b.Rounds[len(b.Rounds)-1]
    for index, player := range r {
        if player.Name == user {
            opponentIndex := index - 1 + ((1 - (index % 2)) * 2)
            if opponentIndex > len(r)-1 {
                opponentA := r[index-1]
                opponentB := r[index-2]
                if b.HasPlayed(r[index-1].Name) {
                    if opponentA.Wins > opponentB.Wins {
                        return opponentA.Name, nil
                    } else {
                        return opponentB.Name, nil
                    }
                }
                return fmt.Sprintf("Winner of (%s vs %s)", opponentA.Name, opponentB.Name), nil
            }
            if b.HasPlayed(user) && b.GetOpponent(r[len(r)-1].Name) == user {
                return r[len(r)-1].Name, nil
            }
            return r[opponentIndex].Name, nil
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