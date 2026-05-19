package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

type Spinner struct {
	stop chan struct{}
	done chan struct{}
}

func StartSpinner(msg string) *Spinner {
	s := &Spinner{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	fmt.Print("\033[?25l") // hide cursor
	go func() {
		defer func() {
			fmt.Print("\033[2K\033[1G\033[?25h") // clear line, restore cursor
			close(s.done)
		}()
		for i := 0; ; i++ {
			select {
			case <-s.stop:
				return
			default:
				fmt.Printf("\033[2K\033[1G%s %s", spinnerStyle.Render(spinnerFrames[i%len(spinnerFrames)]), msg)
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return s
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.done
}
