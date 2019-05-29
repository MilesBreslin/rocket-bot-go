package rocket

import (
    "fmt"
    "strings"
    "time"
)

type message struct {
    IsNew           bool
    IsMention       bool
    IsEdited        bool
    IsMe            bool
    Id              string
    UserName        string
    UserId          string
    RoomName        string
    RoomId          string
    Text            string
    Timestamp       time.Time
    UpdatedAt       time.Time
    Reactions       map[string] []string
    Attachments     []attachment
    rocketCon       rocketCon
}

type attachment struct {
    Description     string
    Title           string
    Type            string
    Link            string
}

func (rock *rocketCon) handleMessageObject(obj map[string] interface{}) message {
    var msg message
    msg.rocketCon = *rock
    msg.IsNew = true
    _, msg.IsEdited = obj["editedAt"]
    if msg.IsEdited {
        msg.IsNew = false
    }
    msg.Id = obj["_id"].(string)
    msg.Text = obj["msg"].(string)
    msg.RoomId = obj["rid"].(string)
    msg.UserId = obj["u"].(map[string] interface{})["_id"].(string)
    msg.UserName = obj["u"].(map[string] interface{})["username"].(string)

    if attachments, ok := obj["attachments"]; ok && attachments != nil {
        msg.Attachments = make([]attachment,0)
        for _, val := range attachments.([]interface{}) {
            var attach attachment
            attach.Description = val.(map[string] interface{})["description"].(string)
            attach.Title = val.(map[string] interface{})["title"].(string)
            attach.Link = val.(map[string] interface{})["title_link"].(string)
            attach.Type = val.(map[string] interface{})["type"].(string)
            msg.Attachments = append(msg.Attachments, attach)
        }
    }

    if msg.UserId == rock.UserId {
        msg.IsMe = true
    }

    if len(msg.Text) > len(rock.UserName)+2 {
        if string(strings.ToLower(msg.Text)[:len(rock.UserName)+2]) == fmt.Sprintf("@%s ", strings.ToLower(rock.UserName)) {
            msg.IsMention = true
        }
    }

    if _, ok := obj["unread"]; !ok {
        msg.IsNew = false
    }

    if _, ok := obj["urls"]; ok {
        if _, ok := obj["urls"].([]interface{})[0].(map[string] interface{})["meta"]; ok {
            msg.IsNew = false
        }
    }

    if react, ok := obj["reactions"]; ok {
        msg.IsNew = false
        msg.Reactions = make(map[string][]string)
        for emote, val := range react.(map[string]interface{}) {
            for _, username := range val.(map[string]interface{})["usernames"].([]interface{}) {
                msg.Reactions[emote] = append(msg.Reactions[emote], username.(string))
            }
        }
    }
    unixTs := obj["ts"].(map[string]interface{})["$date"].(float64)
    msg.Timestamp = time.Unix(int64(unixTs/1000),0)

    if _, ok := obj["_updatedAt"]; ok {
        unixUp := obj["_updatedAt"].(map[string]interface{})["$date"].(float64)
        msg.UpdatedAt = time.Unix(int64(unixUp/1000),0)
    }

    if val, ok := rock.channels[msg.RoomId]; ok {
        msg.RoomName = val
    }

    fmt.Println(msg.RoomName)

    return msg
}

func (msg *message) Reply(text string) {
    msg.rocketCon.SendMessage(msg.RoomId, text)
}

func (msg *message) KickUser() {
    msg.Reply("/kick @"+msg.UserName)
}

func (msg *message) React(emoji string) (error) {
    return msg.rocketCon.React(msg.Id, emoji)
}

func (msg *message) GetNoMention() (string) {
    r := msg.Text
    if len(msg.Text) > 2 && msg.IsMention {
        r = string(strings.ToLower(msg.Text)[len(msg.rocketCon.UserName)+2:])
    }
    return r
}