package chat

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/gaurav-gosain/gollama/internal/client"
	"github.com/gaurav-gosain/gollama/internal/image"
	"github.com/gaurav-gosain/gollama/internal/roles"
	oapi "github.com/ollama/ollama/api"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/term"
)

type (
	Type              string
	StreamChunk       string
	FinishedStreaming bool
)

var (
	purple = lipgloss.Color("#8839ef")
	teal   = lipgloss.Color("#00baba")
	cream  = lipgloss.Color("#FFFDF5")
	gray   = lipgloss.Color("#aaaaaa")
	black  = lipgloss.Color("#000000")
)

var HighlightStyle = lipgloss.NewStyle().
	Background(purple).
	Bold(true).
	Padding(0, 1)

var HighlightActiveStyle = lipgloss.NewStyle().
	Background(teal).
	Foreground(black).
	Bold(true).
	Padding(0, 1)

var HighlightForegroundStyle = lipgloss.NewStyle().
	Foreground(purple).
	Bold(true)

var DisabledHighlightStyle = lipgloss.NewStyle().
	Foreground(gray).
	Bold(true)

var RoundedBorder = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder())

type ContentType string

type ChatMessage struct {
	CreatedAt time.Time
	Role      string
	Message   string
	Images    []string
}

func EncodeGob(w io.Writer, messages *[]ChatMessage) error {
	if err := gob.NewEncoder(w).Encode(messages); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

func DecodeGob(r io.Reader, messages *[]ChatMessage) error {
	if err := gob.NewDecoder(r).Decode(messages); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

type Chat struct {
	imagepicker          filepicker.Model
	promptForm           *huh.Form
	Glamour              *glamour.TermRenderer
	modelName            string
	attachedImage        string
	viewport             viewport.Model
	chatState            []string
	ChatHistory          []ChatMessage
	ChatSettings         client.Chat
	width                int
	highlightedChatIndex int
	height               int
	isMultiModal         bool
	streaming            bool
	pickingImage         bool
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func GenerateChatID() string {
	// Creating UUID Version 4
	// panic on error
	u1 := uuid.Must(uuid.NewV4(), nil)

	// check db if exists
	count := 0
	err := client.GollamaInstance.DB.Get(&count, "SELECT COUNT(*) FROM chats WHERE id = ?", u1.String())
	if err != nil {
		panic(err)
	}

	if count > 0 {
		return GenerateChatID()
	}

	return u1.String()
}

func NewChat(chatSettings client.Chat) *Chat {
	vp := viewport.New(30, 5)

	fp := filepicker.New()
	fp.AllowedTypes = []string{".png", ".jpeg", ".jpg"}
	fp.CurrentDirectory, _ = os.UserHomeDir()
	// fp.Height = 10
	fp.AutoHeight = true

	highlightedChatIndex := 0

	var chatHistory []ChatMessage

	if !chatSettings.IsAnonymous {
		// make sure the chat history file exists
		if _, err := os.Stat(filepath.Join(
			xdg.DataHome,
			"gollama",
			"chats",
			chatSettings.ID+".gob",
		)); os.IsNotExist(err) {
			// create the chat history file
			file, err := os.Create(filepath.Join(
				xdg.DataHome,
				"gollama",
				"chats",
				chatSettings.ID+".gob",
			))
			if err != nil {
				panic(err)
			}
			defer file.Close() //nolint:errcheck
		} else {
			// read the chat history file
			file, err := os.Open(filepath.Join(
				xdg.DataHome,
				"gollama",
				"chats",
				chatSettings.ID+".gob",
			))
			if err != nil {
				panic(err)
			}
			defer file.Close() //nolint:errcheck

			if err := DecodeGob(file, &chatHistory); err != nil {
				panic(err)
			}

			if len(chatHistory) > 0 {
				highlightedChatIndex = len(chatHistory) - 1
			}
		}
	}

	return &Chat{
		modelName:            chatSettings.ModelName,
		imagepicker:          fp,
		ChatHistory:          chatHistory,
		isMultiModal:         chatSettings.IsMultiModal,
		viewport:             vp,
		ChatSettings:         chatSettings,
		highlightedChatIndex: highlightedChatIndex,
	}
}

func (chat *Chat) resetPrompt(
	bg, fg lipgloss.Color,
	placeholder string,
	titleStyle lipgloss.Style,
	focus bool,
) []tea.Cmd {
	cmds := []tea.Cmd{}

	textField := huh.NewText().
		Key("message").
		Title(titleStyle.Render("Chat with ") + makeRounded(lipgloss.
			NewStyle().
			Background(bg).
			Foreground(fg).
			Padding(0, 1).
			Bold(true).
			Render(chat.modelName), bg)).
		Placeholder(placeholder).
		Validate(func(s string) error {
			if focus && len(strings.TrimSpace(s)) == 0 {
				return errors.New("prompt cannot be empty")
			}
			return nil
		}).
		WithHeight(3)

	chat.promptForm = huh.NewForm(
		huh.NewGroup(
			textField,
		),
	).
		WithWidth(max(30, chat.width))

	cmds = append(cmds, chat.promptForm.Init())

	if !focus {
		cmds = append(cmds, textField.Blur())
	}

	return cmds
}

func (chat *Chat) Init() tea.Cmd {
	physicalWidth, physicalHeight, _ := term.GetSize(int(os.Stdout.Fd()))

	chat.width = physicalWidth - 2
	chat.height = physicalHeight

	cmds := chat.resetPrompt(
		purple,
		cream,
		"Type your message here...",
		HighlightForegroundStyle,
		true,
	)

	cmds = append(cmds, chat.imagepicker.Init())

	cmds = append(cmds, chat.Resize())

	return tea.Batch(
		cmds...,
	)
}

func (chat *Chat) redrawViewport() {
	if len(chat.chatState) == 0 {
		return
	}

	state := []string{}

	for i := range chat.ChatHistory {
		if i == chat.highlightedChatIndex {
			state = append(state, chat.getMessageBubble(chat.ChatHistory[chat.highlightedChatIndex], true))
		} else {
			state = append(state, chat.chatState[i])
		}

		if chat.ChatHistory[i].Role == roles.ASSISTANT {
			state = append(state,
				lipgloss.
					NewStyle().
					Padding(0, 1).
					Foreground(gray).
					Render(
						humanize.
							Time(
								chat.ChatHistory[i].CreatedAt,
							),
					),
			)
		}
	}

	chat.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Top, state...))
}

func (chat *Chat) updateViewport() {
	if len(chat.chatState) == 0 {
		return
	}
	chat.redrawViewport()

	chat.viewport.GotoBottom()
}

func CenterString(str string, width int, color lipgloss.Color) string {
	spaces := int(float64(width-lipgloss.Width(str)) / 2)
	fg := lipgloss.NewStyle().Foreground(color)
	spacesEnd := width - (spaces + lipgloss.Width(str))
	if spacesEnd < 0 || spaces < 0 {
		return ""
	}
	return fg.
		Render("╭"+strings.Repeat("─", spaces)) +
		str +
		fg.
			Render(strings.Repeat("─", spacesEnd)+"╮")
}

func makeRounded(content string, color lipgloss.Color) string {
	style := lipgloss.NewStyle().Foreground(color).Render
	content = style("") + content + style("")
	return content
}

func addToBorder(content string, border string, color lipgloss.Color) string {
	width := lipgloss.Width(content) - 2
	border = makeRounded(border, color)
	centered := CenterString(border, width, color)
	return lipgloss.JoinVertical(lipgloss.Top, centered, content)
}

func (chat *Chat) getMessageBubble(msg ChatMessage, isSelected bool) string {
	align := lipgloss.Right
	title := msg.Role
	body := msg.Message

	padding := []int{}
	if msg.Role == roles.ASSISTANT {
		align = lipgloss.Left
		title = chat.modelName

		if msg.Message == "" {
			body = DisabledHighlightStyle.
				Padding(1).
				Render(fmt.Sprintf("Waiting for %s...", chat.modelName))
		}

		if !chat.streaming {
			var err error
			body, err = chat.Glamour.Render(msg.Message)
			if err != nil {
				body = msg.Message
			}
			padding = []int{1, 0, 0, 0}
		}
	}

	// TODO: fix this
	for i := range msg.Images {

		vpImg := image.GetImageMatrix(
			msg.Images[i],
			chat.viewport.Width,
			chat.viewport.Height,
		)

		vpImgStr := []string{}
		for _, row := range vpImg {
			vpImgStr = append(vpImgStr, lipgloss.JoinHorizontal(lipgloss.Left, row...))
		}

		vpImgStr = append(vpImgStr, body)

		body = lipgloss.JoinVertical(lipgloss.Left, vpImgStr...)
		padding = []int{1, 2, 0, 2}
	}

	borderColor := purple
	titleStyle := HighlightStyle
	if isSelected {
		borderColor = teal
		titleStyle = HighlightActiveStyle
	}

	alignText := lipgloss.Left
	if msg.Role == roles.USER {
		if lipgloss.Width(body) < len(title)+4 || !strings.Contains(body, "\n") {
			alignText = lipgloss.Center
		}
	}

	width := chat.width / 2

	if chat.width < 80 {
		width = chat.width - 6
	}

	width = min(width, max(len(title)+4, lipgloss.Width(body)+2))

	return lipgloss.NewStyle().
		Width(chat.width).
		Align(align).
		Render(
			addToBorder(
				RoundedBorder.
					Border(lipgloss.RoundedBorder(), false, true, true).
					Width(width).
					Align(alignText).
					Padding(padding...).
					Foreground(cream).
					BorderForeground(borderColor).
					Render(body),
				titleStyle.Render(title),
				borderColor),
		)
}

func (chat *Chat) sendMessage(prompt string, role string) tea.Cmd {
	msg := strings.TrimSpace(prompt)
	if len(msg) == 0 && role == roles.USER {
		return nil
	}

	images := []string{}
	if chat.attachedImage != "" {
		images = append(images, chat.attachedImage)
	}

	currentMessage := ChatMessage{
		Role:      role,
		Message:   msg,
		Images:    images,
		CreatedAt: time.Now(),
	}

	chat.ChatHistory = append(chat.ChatHistory, currentMessage)

	chat.highlightedChatIndex = len(chat.ChatHistory) - 1

	msgBubble := chat.getMessageBubble(currentMessage, false)
	chat.chatState = append(chat.chatState, msgBubble)

	chat.updateViewport()

	chat.attachedImage = ""

	// TODO: ollama call
	if role == roles.USER {
		chat.streaming = true
		chat.sendMessage("", roles.ASSISTANT)
		return func() tea.Msg {
			ctx := context.Background()

			chatHistory := []oapi.Message{}

			// check if the system message is set
			if strings.TrimSpace(chat.ChatSettings.SystemMessage) != "" {
				chatHistory = []oapi.Message{
					{
						Role:    roles.SYSTEM,
						Content: chat.ChatSettings.SystemMessage,
					},
				}
			}

			for _, msg := range chat.ChatHistory {
				imageData := []oapi.ImageData{}
				for _, img := range msg.Images {
					expandedPath, err := image.ExpandPath(img)
					if err != nil {
						continue
					}
					imgData, err := os.ReadFile(expandedPath)
					if err == nil {
						imageData = append(imageData, imgData)
					}
				}

				chatHistory = append(chatHistory, oapi.Message{
					Role:    msg.Role,
					Content: msg.Message,
					Images:  imageData,
				})
			}

			chatRequest := oapi.ChatRequest{
				Model:    chat.modelName,
				Messages: chatHistory,
			}

			client.GollamaInstance.API.Client.Chat(ctx, &chatRequest, func(response oapi.ChatResponse) error {
				// send the response to the bubbletea channel here...
				client.GollamaInstance.Program.Send(StreamChunk(response.Message.Content))
				return nil
			})
			return FinishedStreaming(true)
		}
	}

	return nil
}

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

func helpView() string {
	helpViewStr := "←/→: Navigate • Enter: Select File • ctrl+o: Return to chat"
	return helpStyle(helpViewStr)
}

func (m *Chat) ImagePickerView() string {
	var s strings.Builder
	s.WriteString("\n  ")
	s.WriteString("Pick a file:")
	s.WriteString("\n\n" + m.imagepicker.View() + "\n")
	s.WriteString(helpView())
	return s.String()
}

func (chat *Chat) Resize() tea.Cmd {
	chat.promptForm = chat.promptForm.WithWidth(chat.width)
	// chat.imagepicker.AutoHeight = true

	width := chat.width / 2

	if chat.width < 80 {
		width = chat.width - 4
	}

	// TODO: check if error
	chat.Glamour, _ = glamour.NewTermRenderer(
		glamour.WithStandardStyle("dracula"),
		glamour.WithWordWrap(width),
		// glamour.WithPreservedNewLines(),
	)

	h := lipgloss.Height(chat.promptForm.View())

	// expensive
	chat.chatState = []string{}

	for _, msg := range chat.ChatHistory {
		if msg.Role != roles.SYSTEM {
			msgBubble := chat.getMessageBubble(msg, false)
			chat.chatState = append(chat.chatState, msgBubble)
		}
	}

	chat.viewport.Width = chat.width
	chat.viewport.Height = chat.height - h - 3
	if chat.viewport.Height < 0 {
		chat.viewport.Height = 0
	}
	chat.updateViewport()

	return nil
}

func (chat *Chat) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// var vpCmd tea.Cmd
	// chat.viewport, vpCmd = chat.viewport.Update(msg)
	// cmds = append(cmds, vpCmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return chat, tea.Quit
		}

		if chat.pickingImage && msg.String() == "ctrl+o" {
			chat.pickingImage = false
			return chat, nil
		}

		if !chat.streaming && !chat.pickingImage {
			switch msg.String() {
			case "ctrl+c", "esc":
				return chat, tea.Quit
			case "ctrl+o":
				if chat.isMultiModal {
					chat.pickingImage = true
					return chat, nil
				}
			case "ctrl+x":
				chat.attachedImage = ""
				return chat, nil
			case "ctrl+p":
				chat.highlightedChatIndex--
				if chat.highlightedChatIndex < 0 {
					chat.highlightedChatIndex = 0
				}
				chat.redrawViewport()
				if chat.highlightedChatIndex == 0 {
					chat.viewport.SetYOffset(0)
				} else {
					h := lipgloss.Height(
						lipgloss.JoinVertical(
							lipgloss.Left,
							chat.chatState[:chat.highlightedChatIndex]...,
						),
					)
					chat.viewport.SetYOffset(h)
				}
			case "ctrl+n":
				chat.highlightedChatIndex++
				if chat.highlightedChatIndex >= len(chat.ChatHistory) {
					chat.highlightedChatIndex = len(chat.ChatHistory) - 1
				}
				chat.redrawViewport()
				if chat.highlightedChatIndex == 0 {
					chat.viewport.SetYOffset(0)
				} else {
					h := lipgloss.Height(lipgloss.JoinVertical(lipgloss.Left, chat.chatState[:chat.highlightedChatIndex]...))
					chat.viewport.SetYOffset(h)
				}
			case "ctrl+u":
				chat.viewport.HalfViewUp()
			case "ctrl+d":
				chat.viewport.HalfViewDown()
			case "ctrl+up":
				chat.viewport.LineUp(1)
			case "ctrl+down":
				chat.viewport.LineDown(1)
			}
		}
	case tea.WindowSizeMsg:
		chat.width = msg.Width - 2
		chat.height = msg.Height
		var cmd tea.Cmd
		chat.imagepicker, cmd = chat.imagepicker.Update(msg)

		cmds = append(cmds, cmd)
		cmds = append(cmds, chat.Resize())

		// recalculate dimensions
		return chat, tea.Batch(cmds...)
	case tea.MouseMsg:
		var cmd tea.Cmd
		chat.viewport, cmd = chat.viewport.Update(msg)
		cmds = append(cmds, cmd)
		return chat, tea.Batch(cmds...)
	case StreamChunk:
		// update the last message in the chat history
		chat.ChatHistory[len(chat.ChatHistory)-1].Message += string(msg)
		chat.chatState[len(chat.chatState)-1] = chat.getMessageBubble(chat.ChatHistory[len(chat.ChatHistory)-1], false)
		chat.updateViewport()
		return chat, nil
	case FinishedStreaming:
		chat.streaming = false
		chat.chatState[len(chat.chatState)-1] = chat.getMessageBubble(chat.ChatHistory[len(chat.ChatHistory)-1], false)
		chat.updateViewport()
		chat.attachedImage = ""
		return chat, tea.Batch(
			chat.resetPrompt(
				purple,
				cream,
				"Type your message here...",
				HighlightForegroundStyle,
				true,
			)...,
		)
	}

	isKeyMsg := false
	switch msg.(type) {
	case tea.KeyMsg:
		isKeyMsg = true
	}

	if (!chat.pickingImage && !isKeyMsg) || chat.pickingImage {
		var cmd tea.Cmd
		chat.imagepicker, cmd = chat.imagepicker.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if chat.pickingImage {
		if didSelect, path := chat.imagepicker.DidSelectFile(msg); didSelect {
			// Get the path of the selected file.
			chat.pickingImage = false
			chat.attachedImage = path
			return chat, tea.Batch(cmds...)
		}
	} else if !chat.streaming {
		form, cmd := chat.promptForm.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			chat.promptForm = f
			cmds = append(cmds, cmd)
		}

		if chat.promptForm.State == huh.StateCompleted {
			prompt := chat.promptForm.GetString("message")
			streamCmd := chat.sendMessage(prompt, roles.USER)
			resetChatCmd := chat.resetPrompt(
				gray,
				black,
				"Disabled while response is being streamed...",
				DisabledHighlightStyle,
				false,
			)
			cmds = append(cmds, resetChatCmd...)
			cmds = append(cmds, streamCmd)
		}
	}

	return chat, tea.Batch(cmds...)
}

func (chat *Chat) getAttachedImageView() string {
	if chat.attachedImage == "" {
		return ""
	}

	return lipgloss.NewStyle().Width(chat.width).AlignHorizontal(lipgloss.Center).Render(
		HighlightActiveStyle.Render(
			fmt.Sprintf("󰁦 %s ", chat.attachedImage),
		),
	)
}

func (chat *Chat) View() string {
	if chat.pickingImage {
		return chat.ImagePickerView()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		RoundedBorder.Render(chat.viewport.View()),
		chat.getAttachedImageView(),
		chat.promptForm.View(),
	)
}
