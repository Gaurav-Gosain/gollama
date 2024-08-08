package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/gaurav-gosain/gollama/internal/api"
	"github.com/gaurav-gosain/gollama/internal/chat"
	"github.com/gaurav-gosain/gollama/internal/chatpicker"
	"github.com/gaurav-gosain/gollama/internal/client"
	zone "github.com/lrstanley/bubblezone"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func tui() {
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
		var exitReason chatpicker.ExitReason

		// if we have at least one chat, we show the chat picker, else show the new chat screen
		if len(chats) > 0 {
			items := []list.Item{}
			for _, chat := range chats {
				items = append(items, list.Item(chat))
			}
			var chatPicker client.Chat
			chatPicker, exitReason, err = chatpicker.NewChatPicker(items)
			if err != nil {
				panic(err)
			}

			switch exitReason {
			case chatpicker.ExitReasonError:
				fmt.Println("Error")
			case chatpicker.ExitReasonCancel:
				fmt.Println("Cancelled")
			case chatpicker.ExitReasonDeleteChat:
				title := lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("205")).
					Render(chatPicker.ChatTitle)

				deleteChat := true
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title("Exit").
							Description("Do you want to delete " + title + "?").
							Value(&deleteChat),
					),
				).WithProgramOptions(tea.WithAltScreen())

				if err := form.Run(); err != nil {
					panic(err)
				}

				if deleteChat {
					// remove chat from disk
					if err := os.Remove(filepath.Join(
						xdg.DataHome,
						"gollama",
						"chats",
						chatPicker.ID+".gob",
					)); err != nil {
						panic(err)
					}

					// delete chat from db
					if err := client.GollamaInstance.DeleteChat(chatPicker.ID); err != nil {
						panic(err)
					}

					fmt.Println(
						lipgloss.NewStyle().Padding(1, 2).Render(
							fmt.Sprint(
								"Chat ",
								title,
								" deleted successfully",
							),
						),
					)
				}
			case chatpicker.ExitReasonSelect:
				chatSettings = chatPicker
			case chatpicker.ExitReasonNewChat:
				chatSettings, err = chat.NewChatSettingsForm()
				if err != nil {
					panic(err)
				}
			}
		} else {
			exitReason = chatpicker.ExitReasonNewChat
			chatSettings, err = chat.NewChatSettingsForm()
			if err != nil {
				panic(err)
			}
		}

		if exitReason == chatpicker.ExitReasonNewChat || exitReason == chatpicker.ExitReasonSelect {
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

			zone.NewGlobal()

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
		}

		wantToExit := true
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Exit").
					Description("Do you want to exit gollama?").
					Value(&wantToExit),
			),
		)
		// .WithProgramOptions(tea.WithAltScreen())

		if err := form.Run(); err != nil {
			panic(err)
		}

		if wantToExit {
			return
		}
	}
}

func main() {
	tui()
}
