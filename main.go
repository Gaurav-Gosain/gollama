package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/gaurav-gosain/gollama/internal/api"
	"github.com/gaurav-gosain/gollama/internal/chat"
	"github.com/gaurav-gosain/gollama/internal/chatpicker"
	"github.com/gaurav-gosain/gollama/internal/client"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func main() {
	err := client.GollamaInstance.InitDB()
	if err != nil {
		panic(err)
	}

	defer client.GollamaInstance.DB.Close()

	for {
		chats, err := client.GollamaInstance.ListChats()
		if err != nil {
			panic(err)
		}

		var chatSettings client.Chat

		// if we have at least one chat, we show the chat picker, else show the new chat screen
		if len(chats) > 0 {
			items := []list.Item{}
			for _, chat := range chats {
				items = append(items, list.Item(chat))
			}
			chatPicker, exitReason, err := chatpicker.NewChatPicker(items)
			if err != nil {
				panic(err)
			}

			switch exitReason {
			case chatpicker.ExitReasonError:
				fmt.Println("Error")
				return
			case chatpicker.ExitReasonCancel:
				fmt.Println("Cancelled")
				return
			case chatpicker.ExitReasonSelect:
				chatSettings = chatPicker
			case chatpicker.ExitReasonNewChat:
				chatSettings, err = chat.NewChatSettingsForm()
				if err != nil {
					panic(err)
				}
			}

		} else {
			chatSettings, err = chat.NewChatSettingsForm()
			if err != nil {
				panic(err)
			}
		}

		gollamaChat := chat.NewChat(chatSettings)

		p := tea.NewProgram(
			gollamaChat,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		ollamaAPI, err := api.NewOllamaAPI()
		if err != nil {
			panic(err)
		}

		client.GollamaInstance.Connect(ollamaAPI, p)

		var m tea.Model

		if m, err = p.Run(); err != nil {
			panic(err)
		}

		gollamaChat = m.(*chat.Chat)

		if !gollamaChat.ChatSettings.IsAnonymous {
			// dump chat history to .gob file
			file, err := os.Create(filepath.Join(
				xdg.DataHome,
				"gollama",
				"chats",
				gollamaChat.ChatSettings.ID+".gob",
			))
			if err != nil {
				panic(err)
			}
			defer file.Close() //nolint:errcheck

			if err := chat.EncodeGob(file, &gollamaChat.ChatHistory); err != nil {
				panic(err)
			}
		}

		wantToExit := false
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Exit").
					Description("Do you want to exit gollama?").
					Value(&wantToExit),
			),
		).WithProgramOptions(tea.WithAltScreen())

		if err := form.Run(); err != nil {
			panic(err)
		}

		if wantToExit {
			return
		}
	}
}
