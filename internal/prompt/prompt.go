package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

type Prompter struct {
	in    *bufio.Reader
	rawIn io.Reader
	out   io.Writer
}

type Option struct {
	Label string
	Help  string
}

func New(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{
		in:    bufio.NewReader(in),
		rawIn: in,
		out:   out,
	}
}

func (p *Prompter) Text(label, def string) (string, error) {
	if err := p.writePrompt(label, def); err != nil {
		return "", err
	}

	answer, err := p.readAnswer()
	if err != nil {
		return "", err
	}
	if answer == "" {
		return def, nil
	}
	return answer, nil
}

func (p *Prompter) Password(label string) (string, error) {
	if err := p.writePrompt(label, ""); err != nil {
		return "", err
	}

	if file, ok := p.rawIn.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		data, err := term.ReadPassword(int(file.Fd()))
		if _, newlineErr := fmt.Fprintln(p.out); err != nil {
			return "", err
		} else if newlineErr != nil {
			return "", newlineErr
		}
		return string(data), nil
	}

	return p.readLine()
}

func (p *Prompter) Confirm(label string, def bool) (bool, error) {
	defText := "n"
	if def {
		defText = "y"
	}
	if err := p.writePrompt(label, defText); err != nil {
		return false, err
	}

	answer, err := p.readAnswer()
	if err != nil {
		return false, err
	}
	switch strings.ToLower(answer) {
	case "":
		return def, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid confirmation answer %q", answer)
	}
}

func (p *Prompter) Select(label string, options []Option, def int) (int, error) {
	if err := p.writeOptions(label, options); err != nil {
		return 0, err
	}
	if err := p.writePrompt("Choice", strconv.Itoa(def+1)); err != nil {
		return 0, err
	}

	answer, err := p.readAnswer()
	if err != nil {
		return 0, err
	}
	if answer == "" {
		return def, nil
	}

	choice, err := strconv.Atoi(answer)
	if err != nil {
		return 0, fmt.Errorf("invalid selection %q", answer)
	}
	index := choice - 1
	if index < 0 || index >= len(options) {
		return 0, fmt.Errorf("selection %d out of range", choice)
	}
	return index, nil
}

func (p *Prompter) MultiSelect(label string, options []Option, defaults []bool) ([]int, error) {
	if err := p.writeMultiSelectOptions(label, options, defaults); err != nil {
		return nil, err
	}
	if err := p.writePrompt("Choices", ""); err != nil {
		return nil, err
	}

	answer, err := p.readAnswer()
	if err != nil {
		return nil, err
	}
	if answer == "" {
		selected := []int{}
		for i, enabled := range defaults {
			if i >= len(options) {
				break
			}
			if enabled {
				selected = append(selected, i)
			}
		}
		return selected, nil
	}

	parts := strings.Split(answer, ",")
	selected := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		choice, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q", part)
		}
		index := choice - 1
		if index < 0 || index >= len(options) {
			return nil, fmt.Errorf("selection %d out of range", choice)
		}
		selected = append(selected, index)
	}
	return selected, nil
}

func (p *Prompter) readAnswer() (string, error) {
	line, err := p.readLine()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (p *Prompter) readLine() (string, error) {
	line, err := p.in.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (p *Prompter) writePrompt(label, def string) error {
	if def == "" {
		_, err := fmt.Fprintf(p.out, "%s: ", label)
		return err
	}
	_, err := fmt.Fprintf(p.out, "%s [%s]: ", label, def)
	return err
}

func (p *Prompter) writeOptions(label string, options []Option) error {
	if _, err := fmt.Fprintln(p.out, label); err != nil {
		return err
	}
	for i, option := range options {
		if option.Help == "" {
			if _, err := fmt.Fprintf(p.out, "%d. %s\n", i+1, option.Label); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(p.out, "%d. %s - %s\n", i+1, option.Label, option.Help); err != nil {
			return err
		}
	}
	return nil
}

func (p *Prompter) writeMultiSelectOptions(label string, options []Option, defaults []bool) error {
	if _, err := fmt.Fprintln(p.out, label); err != nil {
		return err
	}
	for i, option := range options {
		box := "[ ]"
		if i < len(defaults) && defaults[i] {
			box = "[x]"
		}
		if option.Help == "" {
			if _, err := fmt.Fprintf(p.out, "%s [%d] %s\n", box, i+1, option.Label); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(p.out, "%s [%d] %s - %s\n", box, i+1, option.Label, option.Help); err != nil {
			return err
		}
	}
	return nil
}
