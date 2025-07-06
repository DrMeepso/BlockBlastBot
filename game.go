package main

import (
	"fmt"
	"sync"
)

type GameState struct {
	Board  uint64  // 64 bits representing the 8x8 board, each bit is a block
	Pieces []Piece // 3 pieces, each piece is a 2D slice of bools

	Score int // Score of the game state, can be used to sort game states by score
}

func (gs *GameState) FromBoard(b [][]bool) {
	// Convert a 2D slice of bools to an int64 board representation
	gs.Board = 0
	for y, row := range b {
		for x, cell := range row {
			if cell {
				pos := (y * 8) + x
				gs.Board |= 1 << pos
			}
		}
	}
}

// returns a int64 of a empty game board with the pieces placed at the given x, y coordinates
// the cords are the top left corner of the piece
func (p *Piece) ToGameBoard(x int, y int) uint64 {
	board := uint64(0)

	for i, row := range *p {
		for j, cell := range row {
			if cell {
				pos := (y+i)*8 + (x + j)
				board |= 1 << pos
			}
		}
	}

	return board
}

func (p *Piece) getBounds() (int, int) {
	maxWidth := 0
	maxHeight := len(*p)

	for _, row := range *p {
		width := 0
		for _, cell := range row {
			if cell {
				width++
			}
		}
		if width > maxWidth {
			maxWidth = width
		}
	}

	return maxWidth, maxHeight
}

func (gs *GameState) Penalize() {

	// penalize the game state for every empty block that is neighbored by only full blocks
	// penalize the game state for every full block that is neighbored by only empty blocks

	const emptyBlockPenalty = 5
	const fullBlockPenalty = 10

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pos := y*8 + x
			if gs.Board&(1<<pos) == 0 { // empty block
				// check if all neighbors are full
				fullNeighbors := true
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if (dy == 0 && dx == 0) || (dy != 0 && dx != 0) { // skip self and diagonals
							continue
						}
						nx, ny := x+dx, y+dy
						if nx < 0 || nx >= 8 || ny < 0 || ny >= 8 {
							continue // out of bounds
						}
						if gs.Board&(1<<(ny*8+nx)) == 0 {
							fullNeighbors = false
							break
						}
					}
					if !fullNeighbors {
						break
					}
				}
				if fullNeighbors {
					gs.Score -= emptyBlockPenalty // penalize for empty block surrounded by full blocks
				}
			} else { // full block
				// check if all neighbors are empty
				emptyNeighbors := true
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if (dy == 0 && dx == 0) || (dy != 0 && dx != 0) { // skip self and diagonals
							continue
						}
						nx, ny := x+dx, y+dy
						if nx < 0 || nx >= 8 || ny < 0 || ny >= 8 {
							continue // out of bounds
						}
						if gs.Board&(1<<(ny*8+nx)) != 0 {
							emptyNeighbors = false
							break
						}
					}
					if !emptyNeighbors {
						break
					}
				}
				if emptyNeighbors {
					gs.Score -= fullBlockPenalty // penalize for full block surrounded by empty blocks
				}
			}
		}
	}

	// calculate the perimeter of the filled blocks, the smaller the perimeter, the better the game state
	perimeter := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pos := y*8 + x
			if gs.Board&(1<<pos) != 0 { // full block
				// check neighbors
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if (dy == 0 && dx == 0) || (dy != 0 && dx != 0) { // skip self and diagonals
							continue
						}
						nx, ny := x+dx, y+dy
						if nx < 0 || nx >= 8 || ny < 0 || ny >= 8 {
							perimeter++ // out of bounds counts as perimeter
						} else if gs.Board&(1<<(ny*8+nx)) == 0 {
							perimeter++ // empty neighbor counts as perimeter
						}
					}
				}
			}
		}
	}

	// penalize the game state for the perimeter, the smaller the perimeter, the better the game state
	gs.Score -= perimeter

}

type Postion struct {
	X int // X coordinate on the board
	Y int // Y coordinate on the board
}
type Move struct {
	Piece      *Piece  // The piece being moved
	PieceIndex int     // Index of the piece in the game state
	To         Postion // The starting position of the piece
}

func (gs *GameState) PlacePiece(piece *Piece, pos Postion) (*GameState, error) {

	// make a copy of the current game state
	newState := &GameState{
		Board:  gs.Board,
		Pieces: make([]Piece, len(gs.Pieces)),
		Score:  gs.Score,
	}

	for i, p := range gs.Pieces {
		newState.Pieces[i] = make(Piece, len(p))
		for j, row := range p {
			newState.Pieces[i][j] = make([]bool, len(row))
			copy(newState.Pieces[i][j], row)
		}
	}

	// check if the piece collides with any existing pieces
	pieceInt := piece.ToGameBoard(pos.X, pos.Y)
	if newState.Board&pieceInt != 0 {
		return nil, fmt.Errorf("piece collides with existing pieces")
	}

	// place the piece on the board
	newState.Board |= pieceInt

	// check if there are any full rows or columns
	removingPieces := make([]int, 0)
	for y := 0; y < 8; y++ {
		fullRow := true
		for x := 0; x < 8; x++ {
			if (newState.Board & (1 << (y*8 + x))) == 0 {
				fullRow = false
				break
			}
		}
		if fullRow {
			removingPieces = append(removingPieces, y)
		}
	}
	for x := 0; x < 8; x++ {
		fullCol := true
		for y := 0; y < 8; y++ {
			if (newState.Board & (1 << (y*8 + x))) == 0 {
				fullCol = false
				break
			}
		}
		if fullCol {
			removingPieces = append(removingPieces, x+8) // offset for columns
		}
	}

	rowsRemoved := 1

	// remove full rows and columns
	for _, index := range removingPieces {
		if index < 8 { // row
			gs.Score += 50 * rowsRemoved // increase score for each row removed
			rowsRemoved++
			for x := 0; x < 8; x++ {
				newState.Board &^= (1 << (index*8 + x))
			}
		} else { // column
			colIndex := index - 8
			gs.Score += 50 * rowsRemoved // increase score for each column removed
			rowsRemoved++
			for y := 0; y < 8; y++ {
				newState.Board &^= (1 << (y*8 + colIndex))
			}
		}
	}

	return newState, nil

}

func (gs *GameState) Print() {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if gs.Board&(1<<(y*8+x)) != 0 {
				// green for filled
				print("🟩")
			} else {
				// black for empty
				print("⬛️")
			}
		}
		println()
	}
}

var Order = [6][3]int{
	{0, 1, 2},
	{0, 2, 1},
	{1, 0, 2},
	{1, 2, 0},
	{2, 0, 1},
	{2, 1, 0},
}

type Job struct {
	OrderIndex int     // Index of the order to use
	FirstPos   Postion // First position to place the piece
	SecondPos  Postion // Second position to place the piece
	ThirdPos   Postion // Third position to place the piece

	Score int // Score of the job, can be used to sort jobs by score
}

func (gs *GameState) FindBestMove() []Move {

	var JobsMutex sync.Mutex
	Jobs := make([]Job, 0)

	for i, order := range Order {
		// Get bounds for pieces in this specific order
		p1w, p1h := gs.Pieces[order[0]].getBounds()
		p2w, p2h := gs.Pieces[order[1]].getBounds()
		p3w, p3h := gs.Pieces[order[2]].getBounds()

		for x1 := 0; x1 <= 8-p1w; x1++ {
			for y1 := 0; y1 <= 8-p1h; y1++ {
				for x2 := 0; x2 <= 8-p2w; x2++ {
					for y2 := 0; y2 <= 8-p2h; y2++ {
						for x3 := 0; x3 <= 8-p3w; x3++ {
							for y3 := 0; y3 <= 8-p3h; y3++ {
								Jobs = append(Jobs, Job{
									OrderIndex: i,
									FirstPos:   Postion{X: x1, Y: y1},
									SecondPos:  Postion{X: x2, Y: y2},
									ThirdPos:   Postion{X: x3, Y: y3},
								})
							}
						}
					}
				}
			}
		}
	}

	println("Total Jobs:", len(Jobs))

	var ValidJobsMutex sync.Mutex
	var ValidWaits sync.WaitGroup
	ValidJobs := make([]Job, 0)

	for i := range make([]struct{}, 1500) {
		ValidWaits.Add(1)
		go func(threadID int) {
			for {
				JobsMutex.Lock()
				var ThisJob Job
				if len(Jobs) == 0 {
					JobsMutex.Unlock()
					break // No more jobs to process
				} else {
					ThisJob = Jobs[0]
				}
				Jobs = Jobs[1:]
				JobsMutex.Unlock()

				var CurrBoard *GameState = &GameState{
					Board:  gs.Board,
					Pieces: make([]Piece, len(gs.Pieces)),
					Score:  gs.Score,
				}
				var IsValid bool = true
				for pieceIndex := 0; pieceIndex < 3; pieceIndex++ {

					var Pos Postion
					if pieceIndex == 0 {
						Pos = ThisJob.FirstPos
					} else if pieceIndex == 1 {
						Pos = ThisJob.SecondPos
					} else if pieceIndex == 2 {
						Pos = ThisJob.ThirdPos
					}

					piece := &gs.Pieces[Order[ThisJob.OrderIndex][pieceIndex]]
					if piece.ToGameBoard(Pos.X, Pos.Y)&CurrBoard.Board != 0 {
						CurrBoard.Score = -1000000000000 // Invalid job, piece collides with existing pieces
						IsValid = false
						break // Skip this job if it collides with existing pieces
					}

					var err error
					CurrBoard, err = CurrBoard.PlacePiece(&gs.Pieces[Order[ThisJob.OrderIndex][pieceIndex]], Pos)
					if err != nil {
						IsValid = false
						break // Skip this job if it fails
					}
				}

				if !IsValid {
					continue // Skip this job if it fails
				} else {

					if CurrBoard == gs {
						fmt.Println("Job failed to place pieces, this should not happen")
						continue // Skip this job if it fails
					}

					CurrBoard.Penalize()
					ThisJob.Score = CurrBoard.Score

					// If we reach here, all pieces were placed successfully
					ValidJobsMutex.Lock()
					ValidJobs = append(ValidJobs, ThisJob)
					ValidJobsMutex.Unlock()

				}
			}
			ValidWaits.Done()
		}(i)
	}

	ValidWaits.Wait()

	println("Valid Jobs:", len(ValidJobs))

	// Sort valid jobs by score in descending order to find the best move
	if len(ValidJobs) == 0 {
		return nil // No valid jobs found
	}
	bestJob := Job{
		Score:      -1000000, // Start with a very low score
		OrderIndex: -1,
	}
	for _, job := range ValidJobs {
		if job.Score > -1000000 {
			if job.Score > bestJob.Score {
				bestJob = job
			}
		}
	}

	bestMoves := make([]Move, 0, 3)
	pieceOrder := Order[bestJob.OrderIndex]

	bestMoves = append(bestMoves, Move{
		Piece:      &gs.Pieces[pieceOrder[0]],
		PieceIndex: pieceOrder[0],
		To:         bestJob.FirstPos,
	})
	bestMoves = append(bestMoves, Move{
		Piece:      &gs.Pieces[pieceOrder[1]],
		PieceIndex: pieceOrder[1],
		To:         bestJob.SecondPos,
	})
	bestMoves = append(bestMoves, Move{
		Piece:      &gs.Pieces[pieceOrder[2]],
		PieceIndex: pieceOrder[2],
		To:         bestJob.ThirdPos,
	})

	println("Best Score:", bestJob.Score)

	return bestMoves

}
