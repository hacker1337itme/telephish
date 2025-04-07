package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"

    "github.com/go-ole/go-ole"
    "github.com/go-ole/go-ole/oleutil"
)

// Update represents an update from the Telegram API.
type Update struct {
    UpdateID int64        `json:"update_id"`
    Message  *TelegramMsg `json:"message"`
}

// TelegramMsg represents a message in Telegram.
type TelegramMsg struct {
    MessageID int64   `json:"message_id"`
    Text      string  `json:"text"`
    Entities  []Entity `json:"entities"` // Entities might contain URL links
}

// Entity represents the different entities in a message (e.g., URLs).
type Entity struct {
    Type   string `json:"type"`
    Offset int    `json:"offset"`
    Length int    `json:"length"`
    URL    string `json:"url,omitempty"` // Only if the entity type is "url"
}

// GetUpdates fetches updates from the Telegram bot.
func GetUpdates(token string) ([]Update, error) {
    resp, err := http.Get(fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var updates struct {
        Ok     bool    `json:"ok"`
        Result []Update `json:"result"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&updates); err != nil {
        return nil, err
    }

    if !updates.Ok {
        return nil, fmt.Errorf("failed to get updates")
    }

    return updates.Result, nil
}

// ExtractURL extracts URL from a message.
func ExtractURL(message *TelegramMsg) string {
    if message.Entities != nil {
        for _, entity := range message.Entities {
            if entity.Type == "url" {
                return entity.URL
            }
        }
    }
    return ""
}

// ShowNotification creates and displays a toast notification.
func ShowNotification(title, message, url string) error {
    // Initialize OLE
    err := ole.CoInitialize(0)
    if err != nil {
        return fmt.Errorf("failed to initialize OLE: %v", err)
    }
    defer ole.CoUninitialize()

    // Create the Toast Notification Manager
    manager, err := oleutil.CreateObject("Windows.UI.Notifications.ToastNotificationManager")
    if err != nil {
        return fmt.Errorf("failed to create ToastNotificationManager: %v", err)
    }
    defer manager.Release() // Release the COM object when done

    // Get the IDispatch interface from the manager
    managerDispatch, err := manager.QueryInterface(ole.IID_IDispatch)
    if err != nil {
        return fmt.Errorf("failed to query IDispatch: %v", err)
    }
    defer managerDispatch.Release() // Release the interface when done

    // Get the Toast Notifier
    notifier, err := oleutil.GetProperty(managerDispatch, "ToastNotifier")
    if err != nil {
        return fmt.Errorf("failed to get ToastNotifier: %v", err)
    }
    defer notifier.Clear() // Release notifier after use

    // Create the toast XML
    toastXML := fmt.Sprintf(`
    <toast>
        <visual>
            <binding template='ToastGeneric'>
                <text>%s</text>
                <text>%s</text>
            </binding>
        </visual>
        <actions>
            <action content='Open browser' arguments='%s' activationType='foreground'/>
        </actions>
    </toast>`, title, message, url)

    // Create a Toast Notification content
    content, err := oleutil.CallMethod(managerDispatch, "GetTemplateContent", 2) // 2 for ToastGeneric
    if err != nil {
        return fmt.Errorf("failed to get template content: %v", err)
    }
    defer content.Clear() // Release content after use

    // Set the InnerXml property of the Toast Notification
    if _, err := oleutil.PutProperty(content.ToIDispatch(), "InnerXml", toastXML); err != nil {
        return fmt.Errorf("failed to set inner XML: %v", err)
    }

    // Display the notification
    if _, err := oleutil.CallMethod(notifier.ToIDispatch(), "Show", content.ToIDispatch()); err != nil {
        return fmt.Errorf("failed to show notification: %v", err)
    }

    return nil
}

func main() {

    token := os.Getenv("TELEGRAM_BOT_TOKEN") // Set this environment variable

    updates, err := GetUpdates(token)
    if err != nil {
        log.Fatalf("Error fetching updates: %v\n", err)
    }

    if len(updates) == 0 {
        log.Println("No new messages.")
        return
    }

    lastUpdate := updates[len(updates)-1]
    if lastUpdate.Message != nil {
        message := lastUpdate.Message
        url := ExtractURL(message)

        if url != "" {
            title := "New Message"
            content := fmt.Sprintf("You received a new message: %s", message.Text)
            if err := ShowNotification(title, content, url); err != nil {
                log.Fatalf("Error showing notification: %v", err)
            }
        } else {
            log.Println("No URL found in the last message.")
        }
    } else {
        log.Println("No message in the last update.")
    }
}
