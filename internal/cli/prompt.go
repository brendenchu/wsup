package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wavykit/wsup/internal/catalogue"
)

// session is how a command reaches its user: interactive only when both stdin
// and stdout are terminals, so pipes, scripts, and CI never block on a question.
type session struct {
	in          io.Reader
	interactive bool
}

// terminalSession inspects the real standard input and the command's output.
func terminalSession(stdout io.Writer) session {
	file, ok := stdout.(*os.File)
	return session{in: os.Stdin, interactive: ok && isTerminal(os.Stdin) && isTerminal(file)}
}

// errCancelled reports end of input during init questions; nothing is written.
var errCancelled = errors.New("init cancelled; nothing was written")

// promptAttempts bounds re-asking after invalid answers.
const promptAttempts = 3

type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

// ask prints a question with its default and returns the answer as a list of
// names (comma- or space-separated). An empty answer returns the default.
// check validates an answer; a rejected answer is explained and asked again.
func (p prompter) ask(question, defaults string, check func([]string) error) ([]string, error) {
	for range promptAttempts {
		if _, err := fmt.Fprintf(p.out, "%s [%s]: ", question, defaults); err != nil {
			return nil, err
		}
		line, err := p.in.ReadString('\n')
		if err != nil && (err != io.EOF || line == "") {
			if err == io.EOF {
				fmt.Fprintln(p.out)
				return nil, errCancelled
			}
			return nil, err
		}
		names := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' })
		if len(names) == 0 {
			names = strings.FieldsFunc(defaults, func(r rune) bool { return r == ',' || r == ' ' })
		}
		if err := check(names); err != nil {
			fmt.Fprintf(p.out, "  %v\n", err)
			continue
		}
		return names, nil
	}
	return nil, fmt.Errorf("no valid answer after %d attempts; nothing was written", promptAttempts)
}

// initAnswers holds what the questions resolved. A nil field means the
// question was not asked and the flag value stands.
type initAnswers struct {
	preset  *string
	agents  []string
	editors []string
}

// askInit asks only the questions that flags left open: preset and editors for
// a workspace, and agents for both starters. collections are the explicit
// --collection additions, validated together with the chosen preset.
func askInit(s session, out io.Writer, kind string, collections []string, asked func(flag string) bool) (initAnswers, error) {
	p := prompter{in: bufio.NewReader(s.in), out: out}
	answers := initAnswers{}
	if kind == "workspace" && !asked("preset") {
		for _, name := range catalogue.PresetNames() {
			module, _ := mustCatalogue().Module(name)
			fmt.Fprintf(out, "  %s: %s\n", name, module.Description)
		}
		fmt.Fprintf(out, "  %s: the base only\n", catalogue.NoPreset)
		names, err := p.ask("Preset", catalogue.DefaultPreset, func(names []string) error {
			if len(names) != 1 {
				return fmt.Errorf("choose one preset")
			}
			_, err := catalogue.ComposeCollections(names[0], collections)
			return err
		})
		if err != nil {
			return initAnswers{}, err
		}
		answers.preset = &names[0]
	}
	for _, question := range []struct {
		flag, label, kind string
		into              *[]string
	}{
		{"agent", "Agents", catalogue.KindAgent, &answers.agents},
		{"editor", "Editors", catalogue.KindEditor, &answers.editors},
	} {
		if asked(question.flag) || (question.kind == catalogue.KindEditor && kind != "workspace") {
			continue
		}
		defaults, _ := catalogue.ComposeAdapters(question.kind, nil)
		names, err := p.ask(question.label+" (or "+catalogue.NoPreset+")", strings.Join(defaults, ", "), func(names []string) error {
			_, err := catalogue.ComposeAdapters(question.kind, names)
			return err
		})
		if err != nil {
			return initAnswers{}, err
		}
		*question.into = names
	}
	return answers, nil
}

func mustCatalogue() *catalogue.Catalogue {
	c, err := catalogue.Load()
	if err != nil {
		panic("embedded module catalogue is invalid: " + err.Error())
	}
	return c
}
