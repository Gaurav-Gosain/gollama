package image

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nfnt/resize"
)

// ExpandPath expands the tilde (~) in a path to the user's home directory
func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}
	return path, nil
}

func GetImageMatrix(path string, physicalWidth int, physicalHeight int) [][]string {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		log.Fatal(err)
	}

	// Open the image file
	file, err := os.Open(expandedPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Decode the image
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	newWidth := physicalWidth / 2
	ratio := float64(img.Bounds().Dy()) / float64(img.Bounds().Dx())

	// if the newHeight based on the newWidth is greater than the physical height, then resize the image to the physical height
	newHeight := int(float64(newWidth) * ratio)
	if newHeight > physicalHeight {
		newHeight = physicalHeight
		newWidth = int(float64(newHeight) / ratio)
	}

	resizedImg := resize.Resize(uint(newWidth), 0, img, resize.NearestNeighbor)

	// Get the bounds of the resized image
	bounds := resizedImg.Bounds()

	imgStringArr := [][]string{}

	// Iterate over the pixels of the resized image
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		imgString := []string{}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Get the pixel color
			color := resizedImg.At(x, y)
			r, g, b, a := color.RGBA()

			if a == 0 {
				imgString = append(imgString, "  ")
				continue
			}
			// Convert to hex string and assign to the 2D slice
			hexColor := fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
			// imgString.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(hexColor)).Render("  "))
			imgString = append(imgString, lipgloss.NewStyle().Background(lipgloss.Color(hexColor)).Render("  "))
		}
		imgStringArr = append(imgStringArr, imgString)
	}

	return imgStringArr
}
