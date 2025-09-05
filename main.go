package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"time"
)

const ADB_PATH = "./platform-tools/adb"

func adbScreenShot() bytes.Buffer {
	// take a screenshot using adb
	var out bytes.Buffer
	cmd := exec.Command(ADB_PATH, "exec-out", "screencap", "-p")
	// use cat to read the output
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr // redirect stderr to the same as the parent process
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	return out
}

type Piece [][]bool

func main() {

	println("Starting Block Blast Bot...")

	// wait for the game to start
	time.Sleep(1 * time.Second)

	for {
		doRound()
		println("Round completed, waiting for next round...")
	}

}

func doRound() {

	// take a screenshot then save it to a file
	out := adbScreenShot()

	os.WriteFile("screenshot.png", out.Bytes(), 0644)

	// read the game board from the screenshot
	// starting at 44, 415 each box is 76x76 pixels with a gap of 3 pixels up and down
	// the board is 8x8 pixels

	img, _, err := image.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		panic(err)
	}

	board := make([][]bool, 8)
	for i := range board {
		board[i] = make([]bool, 8)
	}

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			// calculate the position of the box
			posX := 44 + (x * 76) + (x * 3)
			posY := 415 + (y * 76) + (y * 3)

			// get the color of the pixel at the position
			color := img.At(posX+(76/2), posY+(76/2))

			// print the color
			r, g, b, _ := color.RGBA()
			//println("Box at", x, y, "Color:", r/256, g/256, b/256, a/256)

			// get the luminance of the color
			luminance := (0.299 * float64(r/256)) + (0.587 * float64(g/256)) + (0.114 * float64(b/256))
			board[y][x] = luminance > 40 // threshold for high luminance

		}
	}

	// we now need to extract the pieces that we can play.
	// the three pieces are centered on their axis, we are going to split this area into 3 pieces and then
	// find the bounds of each piece so that we can know what type of piece it is
	pieceContainer := image.Rect(28, 1105, 28+663, 1105+266)
	sliceWidth := (pieceContainer.Dx() - 3) / 3 //

	// make a slice to hold the pieces
	// each piece will be a 2D slice of bools representing the pixels that are
	pieces := make([]Piece, 3)

	// save each one as its own file
	for i := 0; i < 3; i++ {
		pieceRect := image.Rect(28+sliceWidth*i, 1105, sliceWidth*(i+1)+28, pieceContainer.Dy()+1105)

		pi, _ := findPieceBounds(pieceRect, i, img)
		saveRectToFile(img, pi, fmt.Sprintf("piece_%d.png", i))

		// get the width and height of the piece, each block within a piece is 36x36 pixels
		maxWidth := (pi.Dx() / 36) + 1
		maxHeight := (pi.Dy() / 36) + 1

		pieces[i] = make(Piece, maxHeight)
		for y := 0; y < maxHeight; y++ {
			pieces[i][y] = make([]bool, maxWidth)
			for x := 0; x < maxWidth; x++ {
				// calculate the position of the block
				posX := pi.Min.X + (x * 36)
				posY := pi.Min.Y + (y * 36)

				// get the color of the pixel at the position
				color := img.At(posX+(36/2), posY+(36/2))

				// get the luminance of the color
				r, g, b, _ := color.RGBA()
				luminance := (0.299 * float64(r/256)) + (0.587 * float64(g/256)) + (0.114 * float64(b/256))
				pieces[i][y][x] = luminance > 90 || b/256 < 80 || (b/256 < 150 && r/256 > 80) // threshold for high luminance
			}
		}

		// if the piece is empty, skip it
		if pieces[i].ToGameBoard(0, 0) == 0 {
			println("Piece", i, "is empty, trying to find bounds failed")
			return
		}

		// print the piece
		println("Piece", i, ":")
		for y := 0; y < maxHeight; y++ {
			for x := 0; x < maxWidth; x++ {
				if pieces[i][y][x] {
					print("🟩") // green for high luminance
				} else {
					print("🟥") // red for low luminance
				}
			}
			println()
		}
	}

	// now we have the board and the pieces we can now find the best set of 3 moves to play
	state := GameState{
		Board:  0,
		Pieces: pieces,
	}
	state.FromBoard(board)
	state.Print()
	moves := state.FindBestMove()

	for _, move := range moves {
		fmt.Printf("Move: Piece %d at (%d, %d)\n", move.PieceIndex, move.To.X, move.To.Y)
	}

	// the position on the screen to tap to get the piece
	var PiecePositions []Postion = []Postion{
		{X: 150, Y: 1237},
		{X: 360, Y: 1237},
		{X: 570, Y: 1237},
	}

	// use ADB to send the moves to the game
	for _, move := range moves {

		wantedPos := Postion{
			X: 44 + (move.To.X * 76),
			Y: 415 + (move.To.Y * 76),
		}

		w, h := move.Piece.getBounds()
		pieceSize := Postion{
			X: w * 76,
			Y: h * 76,
		}

		endPos := Postion{
			X: wantedPos.X + pieceSize.X/2,
			Y: wantedPos.Y + pieceSize.Y/2,
		}

		startPos := PiecePositions[move.PieceIndex]

		var adjust = 0.73

		var diffx, diffy float64
		diffx = float64(startPos.X-endPos.X) * adjust
		diffy = float64(startPos.Y-endPos.Y) * adjust

		var x2, y2 int
		x2 = int(float64(startPos.X) - diffx)
		y2 = int(float64(startPos.Y) - diffy)

		swipeEnd := Postion{
			X: int(float64(x2) + ((540 - diffx) * 0.04)),
			Y: y2 + int(150),
		}

		fmt.Printf("Swiping from (%d, %d) to (%d, %d)\n", startPos.X, startPos.Y, swipeEnd.X, swipeEnd.Y)

		// get the magnitude of the swipe
		magnitude := math.Sqrt(float64((endPos.X-startPos.X)*(endPos.X-startPos.X) + (endPos.Y-startPos.Y)*(endPos.Y-startPos.Y)))
		swipeTime := magnitude / 400 // adjust the swipe time based on the distance

		// convert to ms
		swipeTime = swipeTime * 1000 // convert to milliseconds

		println("Swipe time:", fmt.Sprintf("%.2f", swipeTime))

		swipeEnd.X += rand.Intn(10) - 5
		swipeEnd.Y += rand.Intn(10) - 5

		exec.Command(ADB_PATH, "shell", "input", "swipe", fmt.Sprintf("%d", startPos.X), fmt.Sprintf("%d", startPos.Y), fmt.Sprintf("%d", swipeEnd.X), fmt.Sprintf("%d", swipeEnd.Y), fmt.Sprintf("%d", int(swipeTime))).Run()

		time.Sleep(time.Duration(swipeTime) * time.Millisecond) // wait for the game to process the move

	}

}

func findPieceBounds(img image.Rectangle, pieceIndex int, masterImg image.Image) (image.Rectangle, error) {

	var x0, x1, y0, y1 int
	// left most pixel, x0
	// right most pixel, x1
	// top most pixel, y0
	// bottom most pixel, y1

	At := func(x, y int) (r, g, b, a uint32) {
		c := masterImg.At(img.Min.X+x, img.Min.Y+y)
		r, g, b, a = c.RGBA()
		return r, g, b, a
	}

	// find x0, scan from left to right top down until we find a pixel with high luminance
	// that now becomes the left bound of the piece
	finished := false
	for x := 0; x < img.Bounds().Dx(); x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			r, g, b, _ := At(x, y)
			//h, s, v := HSV(r, g, b)
			luminance := (0.299 * float64(r/256)) + (0.587 * float64(g/256)) + (0.114 * float64(b/256))
			// BLUE AND PURPLE!!!!!
			if luminance > 90 || b/256 < 80 || (b/256 < 150 && r/256 > 80) {
				if x0 == 0 || x < x0 {
					x0 = x
				}
				if y0 == 0 || y < y0 {
					y0 = y
				}
				if x1 == 0 || x > x1 {
					x1 = x
				}
				if y1 == 0 || y > y1 {
					y1 = y
				}
			}
		}
		if finished {
			break
		}
	}

	// now we have the bounds of the piece
	if x0 >= x1 || y0 >= y1 {
		return image.Rectangle{}, fmt.Errorf("could not find bounds for piece %d", pieceIndex)
	}

	// return the bounds of the piece
	mas := image.Rect(x0+img.Min.X, y0+img.Min.Y, x1+img.Min.X, y1+img.Min.Y)
	return mas, nil

}
