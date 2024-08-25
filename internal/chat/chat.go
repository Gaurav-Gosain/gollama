package chat

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"github.com/adrg/xdg"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/gaurav-gosain/gollama/internal/client"
	"github.com/gaurav-gosain/gollama/internal/roles"
	"github.com/gaurav-gosain/gollama/internal/utils"
	paintbrush "github.com/jordanella/go-ansi-paintbrush"
	zone "github.com/lrstanley/bubblezone"
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
	notif  = lipgloss.Color("#ff9900")
	teal   = lipgloss.Color("#00baba")
	cream  = lipgloss.Color("#FFFDF5")
	gray   = lipgloss.Color("#aaaaaa")
	black  = lipgloss.Color("#000000")
)

var HighlightStyle = lipgloss.NewStyle().
	Background(purple).
	Bold(true).
	Padding(0, 1)

var NotificationStyle = lipgloss.NewStyle().
	Background(notif).
	Foreground(black).
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

var layoutStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	AlignVertical(lipgloss.Center)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(1).Render

type ContentType string

type ChatMessage struct {
	CreatedAt time.Time
	Role      string
	Message   string
	Images    []string
}

var (
	LEFT_HALF_CIRCLE  string = string(0xe0b6)
	RIGHT_HALF_CIRCLE string = string(0xe0b4)
	BORDER_TOP_LEFT   string = string(0x256d)
	BORDER_TOP_RIGHT  string = string(0x256e)
	BORDER_HORIZONTAL string = string(0x2500)
)

// Helper function to center a string within a given width (top rounded border)
func CenterString(str string, width int, color lipgloss.Color) string {
	spaces := int(float64(width-lipgloss.Width(str)) / 2)
	fg := lipgloss.NewStyle().Foreground(color)
	spacesEnd := width - (spaces + lipgloss.Width(str))
	if spacesEnd < 0 || spaces < 0 {
		return ""
	}
	return fg.Render(BORDER_TOP_LEFT+strings.Repeat(BORDER_HORIZONTAL, spaces)) +
		str +
		fg.Render(strings.Repeat(BORDER_HORIZONTAL, spacesEnd)+BORDER_TOP_RIGHT)
}

// Adds rounded semi-circles on either side of the provided content, in the
// same color of the background
func makeRounded(content string, color lipgloss.Color) string {
	style := lipgloss.NewStyle().Foreground(color).Render
	content = style(LEFT_HALF_CIRCLE) + content + style(RIGHT_HALF_CIRCLE)
	return content
}

// Adds a border to the provided content, with the provided border and color
// and marks (bubblezone) the content with the provided id
func addToBorder(content string, border string, color lipgloss.Color, id string) string {
	width := lipgloss.Width(content) - 2
	border = makeRounded(border, color)
	centered := CenterString(border, width, color)
	if id != "" {
		centered = zone.Mark(id, centered)
	}
	return lipgloss.JoinVertical(lipgloss.Top, centered, content)
}

// Encodes the provided messages to gob format and writes it to the writer
func EncodeGob(w io.Writer, messages *[]ChatMessage) error {
	if err := gob.NewEncoder(w).Encode(messages); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

// Decodes the gob-encoded messages from the reader and stores them in the
// provided messages slice
func DecodeGob(r io.Reader, messages *[]ChatMessage) error {
	if err := gob.NewDecoder(r).Decode(messages); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

type Chat struct {
	imagepicker          filepicker.Model
	help                 help.Model
	promptForm           *huh.Form
	Glamour              *glamour.TermRenderer
	modelName            string
	attachedImage        string
	notification         string
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
	helpVisible          bool
	notificationVisible  bool
}

type clearNotificationMsg struct{}

// Helper function to clear the notification after the specified duration
func clearNotificationAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearNotificationMsg{}
	})
}

// Generates a unique chat ID using the UUID Version 4 algorithm
func GenerateChatID() string {
	// Creating UUID Version 4
	u1 := uuid.Must(uuid.NewV4(), nil)

	// check db if exists
	count := 0
	err := client.GollamaInstance.DB.Get(&count, "SELECT COUNT(*) FROM chats WHERE id = ?", u1.String())
	if err != nil {
		utils.PrintError(err, true)
	}

	if count > 0 {
		// recursively call the function until a unique ID is found
		// (rare, only called in the case of a collision)
		return GenerateChatID()
	}

	return u1.String()
}

// Creates a new Chat instance with the provided chat settings
// Loads the chat history from the database if it exists
// Sets up the chat's viewport, prompt form, and image picker
// Sets default values for the chat settings if they don't exist
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
				utils.PrintError(err, true)
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
				utils.PrintError(err, true)
			}
			defer file.Close() //nolint:errcheck

			if err := DecodeGob(file, &chatHistory); err != nil {
				utils.PrintError(err, true)
			}

			// if the chat history is not empty, set the highlighted chat index to the last message
			if len(chatHistory) > 0 {
				highlightedChatIndex = len(chatHistory) - 1
			}
		}
	}

	helpModel := help.New()
	helpModel.ShowAll = true
	helpModel.Styles.FullDesc.UnsetForeground()
	helpModel.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})

	return &Chat{
		modelName:            chatSettings.ModelName,
		imagepicker:          fp,
		ChatHistory:          chatHistory,
		isMultiModal:         chatSettings.IsMultiModal,
		viewport:             vp,
		ChatSettings:         chatSettings,
		highlightedChatIndex: highlightedChatIndex,
		help:                 helpModel,
	}
}

type CopyType string

const (
	CopyLastResponse CopyType = "CopyLastResponse"
	CopyHighlighted  CopyType = "CopyHighlighted"
)

// Helper function to copy the provided content to the clipboard
// Shows a notification with the copied content
func (chat *Chat) CopyToClipboard(content string, copyType CopyType) tea.Cmd {
	// cross-platform clipboard copy
	clipboard.WriteAll(content)

	switch copyType {
	case CopyLastResponse:
		chat.notification = "Copied last response to clipboard"
	case CopyHighlighted:
		chat.notification = "Copied highlighted message to clipboard"
	}

	chat.notificationVisible = true

	return clearNotificationAfter(time.Second * 3)
}

// Renders an image using the provided path and height
// awesome library for rendering images in terminal ;)
func renderImage(path string, height int) string {
	// Create a new AnsiArt instance
	canvas := paintbrush.New()

	imgPath, err := utils.ExpandPath(path)
	if err != nil {
		return "Failed to expand path"
	}

	file, err := os.ReadFile(imgPath)
	if err != nil {
		return "Failed to load image"
	}

	img, _, err := image.Decode(bytes.NewReader(file))
	if err != nil {
		return "Failed to load image"
	}

	canvas.SetImage(img)

	// Add more characters and adjust weights as desired
	weights := map[rune]float64{
		'': .95,
		'': .95,
		'▁': .9,
		'▂': .9,
		'▃': .9,
		'▄': .9,
		'▅': .9,
		'▆': .85,
		'█': .85,
		'▊': .95,
		'▋': .95,
		'▌': .95,
		'▍': .95,
		'▎': .95,
		'▏': .95,
		'●': .95,
		'◀': .95,
		'▲': .95,
		'▶': .95,
		'▼': .9,
		'○': .8,
		'◉': .95,
		'◧': .9,
		'◨': .9,
		'◩': .9,
		'◪': .9,
	}

	canvas.Weights = weights

	// the factor is to compensate for a cell in the terminal not being square (1x1 characters)
	aspectRatio := float64(img.Bounds().Dx()) * 1.15 / float64(img.Bounds().Dy())

	canvas.SetHeight(max(0, height-2))

	canvas.SetAspectRatio(aspectRatio)

	// use all available CPU cores for rendering
	canvas.SetThreads(runtime.NumCPU())

	// Start the rendering process
	canvas.Paint()

	return canvas.GetResult()
}

// Resets the prompt with the provided colors, placeholder, title style, and focus state
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
		WithWidth(max(30, chat.width)).
		WithShowHelp(false)

	cmds = append(cmds, chat.promptForm.Init())

	if !focus {
		cmds = append(cmds, textField.Blur())
	}

	return cmds
}

// Initializes the chat by resetting the prompt and image picker
// Sets the chat's width and height based on the terminal size (initial)
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

// Redraws the chat's viewport
func (chat *Chat) redrawViewport() {
	if len(chat.chatState) == 0 {
		return
	}

	state := []string{}

	for i := range chat.ChatHistory {
		message := chat.chatState[i]
		if i == chat.highlightedChatIndex {
			message = chat.getMessageBubble(chat.ChatHistory[chat.highlightedChatIndex], true, fmt.Sprintf("%d", chat.highlightedChatIndex))
		}

		state = append(state, message)

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

// Updates the chat's viewport by redrawing it and scrolling to the bottom
func (chat *Chat) updateViewport() {
	if len(chat.chatState) == 0 {
		return
	}
	chat.redrawViewport()

	chat.viewport.GotoBottom()
}

func fixMarkdown(msg string) string {
	count := strings.Count(msg, "```")
	if count%2 != 0 {
		lines := strings.Split(msg, "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.HasPrefix(lines[i], "```") {
				lines = append(lines[:i], lines[i+1:]...)
				break
			}
		}
		msg = strings.Join(lines, "\n")
	}
	return msg
}

// Helper function to get the message bubble for the provided message
func (chat *Chat) getMessageBubble(msg ChatMessage, isSelected bool, id string) string {
	align := lipgloss.Right
	title := msg.Role
	body := msg.Message
	isLastMessage := id == fmt.Sprintf("%d", len(chat.ChatHistory)-1)

	padding := []int{0, 2, 0, 0}

	if msg.Role == roles.ASSISTANT {
		align = lipgloss.Left
		title = chat.modelName

		if msg.Message == "" {
			body = fmt.Sprintf("_Waiting for %s..._", chat.modelName)
		}
		if chat.streaming && isLastMessage {
			body = fixMarkdown(body)
		}
	}

	var err error
	width := chat.width / 2

	if chat.width < 80 {
		width = chat.width - 6
	}

	chat.Glamour, _ = glamour.NewTermRenderer(
		glamour.WithStandardStyle("dracula"),
		glamour.WithWordWrap(min(width, lipgloss.Width(body)+4)),
	)
	bodyNew, err := chat.Glamour.Render(body)
	if err != nil {
		bodyNew = body
	}

	body = bodyNew

	// strip the last line if it's empty
	lastLine := strings.Split(body, "\n")[len(strings.Split(body, "\n"))-1]
	if strings.TrimSpace(lastLine) == "" {
		body = strings.TrimSuffix(body, "\n")
	}

	for i := range msg.Images {
		body = lipgloss.JoinVertical(
			lipgloss.Left,
			renderImage(
				msg.Images[i],
				chat.viewport.Height-lipgloss.Height(body),
			),
			"",
			msg.Message,
		)

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

	width = min(width+4, max(len(title)+4, lipgloss.Width(body)+4))

	bubble := addToBorder(
		RoundedBorder.
			Border(lipgloss.RoundedBorder(), false, true, true).
			Align(alignText).
			Width(width).
			Padding(padding...).
			Foreground(cream).
			BorderForeground(borderColor).
			Render(body),
		titleStyle.Render(title),
		borderColor,
		id,
	)

	return lipgloss.NewStyle().
		Width(chat.width).
		Align(align).
		Render(
			bubble,
		)
}

// Function to "send a message" to the chat,
// it takes the prompt, role, and images as input
// and sends the message to the Ollama server using the API
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

	msgBubble := chat.getMessageBubble(
		currentMessage,
		false,
		fmt.Sprintf("%d", len(chat.ChatHistory)-1),
	)
	chat.chatState = append(chat.chatState, msgBubble)

	chat.updateViewport()

	chat.attachedImage = ""

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
					expandedPath, err := utils.ExpandPath(img)
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

func helpView() string {
	helpViewStr := "←/→: Navigate • Enter: Select File • ctrl+o: Return to chat"
	return helpStyle(helpViewStr)
}

func (c *Chat) textAreaHelpView() string {
	helpViewStr := "ctrl+e open editor • enter submit • ctrl+h help"
	if c.isMultiModal {
		helpViewStr = "ctrl+e open editor • enter submit • ctrl+o open image picker • ctrl+h help"
	}
	// helpViewStr := "alt+enter / ctrl+j new line • ctrl+e open editor • enter submit • ctrl+h help"
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

// Resizes the chat's viewport and prompt form based on the chat's width
func (chat *Chat) Resize() tea.Cmd {
	chat.promptForm = chat.promptForm.WithWidth(chat.width)
	// chat.imagepicker.AutoHeight = true

	width := chat.width / 2

	if chat.width < 80 {
		width = chat.width - 4
	}
	h := lipgloss.Height(chat.promptForm.View())

	chat.help.Width = 8 * chat.width / 10

	chat.viewport.Width = chat.width
	chat.viewport.Height = chat.height - h - 4
	if chat.viewport.Height < 0 {
		chat.viewport.Height = 0
	}
	// TODO: check if error
	chat.Glamour, _ = glamour.NewTermRenderer(
		glamour.WithStandardStyle("dracula"),
		glamour.WithWordWrap(width),
		// glamour.WithPreservedNewLines(),
	)

	// TODO: think of a better way to do this (maybe use a goroutine?)
	// currently expensive when there are a lot of messages/images
	chat.chatState = []string{}

	for idx, msg := range chat.ChatHistory {
		if msg.Role != roles.SYSTEM {
			msgBubble := chat.getMessageBubble(msg, false, fmt.Sprintf("%d", idx))
			chat.chatState = append(chat.chatState, msgBubble)
		}
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

		if chat.helpVisible {
			switch keypress := msg.String(); keypress {
			case "ctrl+h":
				chat.helpVisible = false
				return chat, nil
			}
			return chat, nil
		}

		if !chat.streaming && !chat.pickingImage {
			switch msg.String() {
			case "ctrl+o":
				if chat.isMultiModal {
					chat.pickingImage = true
					return chat, nil
				}
			case "ctrl+h":
				chat.helpVisible = true
				return chat, nil
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
					h := lipgloss.Height(
						lipgloss.JoinVertical(
							lipgloss.Left,
							chat.chatState[:chat.highlightedChatIndex]...,
						),
					)
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
			case "alt+y":
				cmd := chat.CopyToClipboard(
					chat.ChatHistory[chat.highlightedChatIndex].Message,
					CopyHighlighted,
				)
				cmds = append(cmds, cmd)
				return chat, tea.Batch(cmds...)
			case "ctrl+y":
				cmd := chat.CopyToClipboard(
					chat.ChatHistory[len(chat.ChatHistory)-1].Message,
					CopyLastResponse,
				)
				cmds = append(cmds, cmd)
				return chat, tea.Batch(cmds...)
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
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			for idx := range chat.ChatHistory {
				// Check each item to see if it's in bounds.
				if zone.Get(fmt.Sprintf("%d", idx)).InBounds(msg) {
					chat.highlightedChatIndex = idx
					chat.redrawViewport()
					break
				}
			}
		}

		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonRight {
			cmd := chat.CopyToClipboard(chat.ChatHistory[chat.highlightedChatIndex].Message, CopyHighlighted)
			cmds = append(cmds, cmd)
			return chat, tea.Batch(cmds...)
		}

		return chat, tea.Batch(cmds...)
	case StreamChunk:
		// update the last message in the chat history
		chat.ChatHistory[len(chat.ChatHistory)-1].Message += string(msg)
		chat.chatState[len(chat.chatState)-1] = chat.getMessageBubble(chat.ChatHistory[len(chat.ChatHistory)-1], false, fmt.Sprintf("%d", len(chat.ChatHistory)-1))
		chat.updateViewport()
		return chat, nil
	case clearNotificationMsg:
		chat.notification = ""
		chat.notificationVisible = false
		return chat, nil
	case FinishedStreaming:
		chat.streaming = false
		chat.chatState[len(chat.chatState)-1] = chat.getMessageBubble(chat.ChatHistory[len(chat.ChatHistory)-1], false, fmt.Sprintf("%d", len(chat.ChatHistory)-1))
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

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		RoundedBorder.Render(chat.viewport.View()),
		chat.getAttachedImageView(),
		chat.promptForm.View(),
		chat.textAreaHelpView(),
	)

	if chat.helpVisible {

		if chat.isMultiModal {
			Keys.SetFullHelpKeys(Keys.DefaultFullHelpKeys())
		} else {
			Keys.SetFullHelpKeys(Keys.DefaultFullHelpKeysNonMultiModal())
		}
		// chat.help.ShowAll = true

		content = utils.PlaceOverlay(
			chat.width/10,
			chat.height/10,
			layoutStyle.
				Width(8*chat.width/10).
				Height(8*chat.height/10).
				AlignHorizontal(lipgloss.Center).
				BorderForeground(purple).
				Render(
					HighlightStyle.Render(" Help Menu ")+
						"\n\n"+
						chat.help.View(Keys)+
						"\n\n"+
						fmt.Sprintf("Press %s to close this menu", HighlightStyle.Render(" ctrl+h ")),
				),
			content,
		)
	}

	if chat.notificationVisible {
		content = utils.PlaceOverlay(
			chat.width,
			0,
			addToBorder(
				layoutStyle.
					Border(lipgloss.RoundedBorder(), false, true, true).
					Padding(1, 2).
					BorderForeground(notif).
					Render(
						chat.notification,
					),
				NotificationStyle.Render(" Notification "),
				notif,
				"",
			),
			content,
		)
	}

	return zone.Scan(content)
}
