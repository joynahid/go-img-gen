package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"

	"github.com/fogleman/gg"
	"github.com/gin-gonic/gin"
)

type Color struct {
	R uint8 `json:"r" default:"0"`
	G uint8 `json:"g" default:"0"`
	B uint8 `json:"b" default:"0"`
	A uint8 `json:"a" default:"255"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type TextAlign string

const (
	Left   TextAlign = "left"
	Center TextAlign = "center"
	Right  TextAlign = "right"
)

type StyledText struct {
	Text     string   `json:"text"`
	Color    Color    `json:"color"`
	Font     string   `json:"font"`
	SizePx   float64  `json:"sizePx"`
	Position Position `json:"position"`
}

// Set default values for LineSpacingPx
type MultiLineText struct {
	StyledText    `json:"styledText"`
	WrapWidthPx   float64   `json:"wrapWidthPx" binding:"required"`
	LineSpacingPx float64   `json:"lineSpacingPx" default:"1.5"`
	Align         TextAlign `json:"align"`
}

type Rectangle struct {
	Position Position `json:"position"`
	Color    Color    `json:"color"`
	WidthPx  float64  `json:"widthPx"`
	HeightPx float64  `json:"heightPx"`
}

type ImgRequest struct {
	Name            string          `json:"name"`
	WidthPx         int             `json:"widthPx" binding:"required"`
	HeightPx        int             `json:"heightPx" binding:"required"`
	BgImgPath       string          `json:"bgImgPath"`
	BgColor         Color           `json:"bgColor"`
	SingleLineTexts []StyledText    `json:"singleLineTexts"`
	MultiLineTexts  []MultiLineText `json:"multiLineTexts"`
	Rectangles      []Rectangle     `json:"rectangles"`
	Quality         int             `json:"quality"`
}

func GenerateImage(request ImgRequest) *bytes.Buffer {
	newImg := gg.NewContext(request.WidthPx, request.HeightPx)

	if request.BgImgPath != "" {
		img, err := gg.LoadImage(request.BgImgPath)
		if err != nil {
			panic(err)
		}

		// Paste image to new image
		newImg.DrawImage(img, 0, 0)
	} else if request.BgColor != (Color{}) {
		newImg.SetColor(color.RGBA{request.BgColor.R, request.BgColor.G, request.BgColor.B, request.BgColor.A})
		newImg.Clear()
	} else {
		panic("No background image or color provided")
	}

	for _, text := range request.SingleLineTexts {

		fontFace, fontFaceErr := gg.LoadFontFace(text.Font, text.SizePx)
		if fontFaceErr != nil {
			panic(fontFaceErr)
		}

		newImg.SetFontFace(fontFace)
		newImg.SetColor(color.RGBA{text.Color.R, text.Color.G, text.Color.B, text.Color.A})
		newImg.DrawString(text.Text, text.Position.X, text.Position.Y)
	}

	for _, rectangle := range request.Rectangles {
		strokePattern := gg.NewSolidPattern(color.RGBA{rectangle.Color.R, rectangle.Color.G, rectangle.Color.B, rectangle.Color.A})

		newImg.SetStrokeStyle(strokePattern)
		newImg.SetLineWidth(5)

		newImg.DrawRectangle(rectangle.Position.X, rectangle.Position.Y, rectangle.WidthPx, rectangle.HeightPx)
		newImg.Stroke()
		newImg.Fill()
	}

	for _, text := range request.MultiLineTexts {
		fontFace, fontFaceErr := gg.LoadFontFace(text.Font, text.SizePx)
		if fontFaceErr != nil {
			panic(fontFaceErr)
		}

		newImg.SetFontFace(fontFace)
		newImg.SetColor(color.RGBA{text.Color.R, text.Color.G, text.Color.B, text.Color.A})

		var align gg.Align

		switch text.Align {
		case Left:
			align = gg.AlignLeft
		case Center:
			align = gg.AlignCenter
		case Right:
			align = gg.AlignRight
		}

		newImg.DrawStringWrapped(
			text.Text,
			text.Position.X,
			text.Position.Y,
			0,                  // ax: horizontal alignment (0 = left)
			0,                  // ay: vertical alignment (0 = top)
			text.WrapWidthPx,   // width before wrapping
			text.LineSpacingPx, // line spacing
			align,              // text alignment within the box
		)
	}

	// Return Base64 encoded image
	buff := new(bytes.Buffer)
	jpeg.Encode(buff, newImg.Image(), &jpeg.Options{Quality: request.Quality})

	return buff
}

func BuildFontFaceList() []string {
	// Glob all font files in gfonts folder
	files, err := filepath.Glob("gfonts/**/*.ttf")
	if err != nil {
		panic(err)
	}

	fontFaces := []string{}

	for _, file := range files {
		fmt.Println(file)
		fontFaces = append(fontFaces, file)
	}

	return fontFaces
}

func Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token != os.Getenv("API_KEY") {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
		}
	}
}

func main() {
	fontFaces := BuildFontFaceList()
	opts := gin.OptionFunc(func(engine *gin.Engine) {
		engine.Use(gin.Recovery())
	})

	router := gin.New(opts)

	router.Use(Authenticate())

	router.GET("/font-faces", func(c *gin.Context) {
		c.JSON(200, gin.H{"fontFaces": fontFaces})
	})

	router.POST("/generate", func(c *gin.Context) {
		var request ImgRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		for _, text := range request.SingleLineTexts {
			if !slices.Contains(fontFaces, text.Font) {
				c.JSON(400, gin.H{"error": "Font not found"})
				return
			}
		}

		image := GenerateImage(request)
		if image == nil {
			c.JSON(500, gin.H{"error": "Failed to generate image"})
			return
		}

		// Stream image to client
		c.Data(200, "image/jpeg", image.Bytes())
	})

	router.Run(":8080")
}
