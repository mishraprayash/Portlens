package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mishraprayash/Portlens/internal/detect"
	"github.com/mishraprayash/Portlens/internal/exitcode"
	"github.com/mishraprayash/Portlens/internal/platform"
)

// completionSubcommand handles `portlens completion <bash|zsh|fish>`.
type completionSubcommand struct{}

func (c *completionSubcommand) Name() string      { return "completion" }
func (c *completionSubcommand) Aliases() []string { return nil }
func (c *completionSubcommand) Description() string {
	return "Generate shell autocompletion script (bash, zsh, fish)"
}
func (c *completionSubcommand) Run(_ context.Context, args []string, _ []string, stdout, stderr io.Writer, _ io.Reader) int {
	return runCompletion(args, stdout, stderr)
}

func runCompletion(args []string, stdout, stderr io.Writer) int {
	shell := "zsh"
	if len(args) > 0 {
		shell = strings.ToLower(strings.TrimSpace(args[0]))
	}
	script, ok := renderCompletion(shell)
	if !ok {
		return fail(stderr, exitcode.InvalidArguments, "portlens completion: unsupported shell %q (supported: bash, zsh, fish)\n", shell)
	}
	fmt.Fprint(stdout, script)
	return exitcode.Success
}

// flagDoc documents one root flag for shell completion. flagDocs is the only
// copy of the flag inventory used by completion; a test asserts it matches
// registerFlags so the two cannot drift.
type flagDoc struct {
	long  string
	short string
	desc  string
}

var flagDocs = []flagDoc{
	{long: "tree", short: "t", desc: "Show process hierarchy"},
	{long: "connections", short: "n", desc: "Show network connections"},
	{long: "json", short: "j", desc: "JSON output"},
	{long: "kill", short: "k", desc: "Gracefully terminate owning process"},
	{long: "force", short: "f", desc: "Force SIGKILL"},
	{long: "restart", short: "r", desc: "Restart process"},
	{long: "open", short: "o", desc: "Open in browser"},
	{long: "yes", short: "y", desc: "Skip confirmations"},
	{long: "no-color", desc: "Plain text output"},
	{long: "no-docker", desc: "Disable container detection"},
	{long: "tcp", desc: "Only show TCP listeners"},
	{long: "all", desc: "Act on all listening ports"},
	{long: "watch", short: "w", desc: "Re-render every interval"},
	{long: "notify", desc: "Desktop notification on state change"},
	{long: "verbose", short: "v", desc: "Full detailed report"},
	{long: "debug", short: "d", desc: "Diagnostic debug logging"},
	{long: "probe", short: "p", desc: "Probe HTTP endpoint for status and title"},
	{long: "interval", desc: "Watch re-render interval in seconds"},
	{long: "pid", desc: "Target process by PID"},
	{long: "name", desc: "Target process by name"},
	{long: "help", short: "h", desc: "Show help"},
	{long: "version", desc: "Print version"},
	{long: "protocol", desc: "Protocol filter (tcp or udp)"},
	{long: "sort", desc: "Sort key (port, process, project, runtime)"},
	{long: "filter", desc: "Filter by process name"},
}

// completionCommand is one completable subcommand name with its description.
type completionCommand struct {
	name string
	desc string
}

// completionCommands enumerates every registered subcommand and alias in
// registration order, so the scripts always match the dispatch registry.
func completionCommands() []completionCommand {
	reg := defaultSubcommandRegistry()
	out := make([]completionCommand, 0, len(reg.commands))
	for _, c := range reg.ordered {
		out = append(out, completionCommand{name: c.Name(), desc: c.Description()})
		for _, a := range c.Aliases() {
			out = append(out, completionCommand{name: a, desc: c.Description()})
		}
	}
	return out
}

// renderCompletion fills the {{COMMANDS}} and {{FLAGS}} placeholders of the
// requested shell's script template. Reports ok=false for unknown shells.
func renderCompletion(shell string) (string, bool) {
	var tmpl string
	switch shell {
	case "bash":
		tmpl = bashCompletionScript
	case "zsh":
		tmpl = zshCompletionScript
	case "fish":
		tmpl = fishCompletionScript
	default:
		return "", false
	}
	s := strings.ReplaceAll(tmpl, "{{COMMANDS}}", completionChunk(shell, true))
	s = strings.ReplaceAll(s, "{{FLAGS}}", completionChunk(shell, false))
	return s, true
}

// completionChunk renders either the subcommand list or the flag list for a
// shell, in that shell's native syntax.
func completionChunk(shell string, commands bool) string {
	if commands {
		switch shell {
		case "bash":
			cmds := completionCommands()
			names := make([]string, len(cmds))
			for i, c := range cmds {
				names[i] = c.name
			}
			return strings.Join(names, " ")
		case "zsh":
			var b strings.Builder
			for i, c := range completionCommands() {
				if i > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(&b, "        '%s:%s'", c.name, c.desc)
			}
			return b.String()
		case "fish":
			var lines []string
			for _, c := range completionCommands() {
				lines = append(lines, fmt.Sprintf("complete -c portlens -n '__fish_use_subcommand' -a '%s' -d '%s'", c.name, c.desc))
			}
			return strings.Join(lines, "\n")
		}
		return ""
	}
	switch shell {
	case "bash":
		var parts []string
		for _, f := range flagDocs {
			parts = append(parts, "--"+f.long)
			if f.short != "" {
				parts = append(parts, "-"+f.short)
			}
		}
		return strings.Join(parts, " ")
	case "zsh":
		var lines []string
		for _, f := range flagDocs {
			lines = append(lines, fmt.Sprintf("        '--%s[%s]'", f.long, f.desc))
			if f.short != "" {
				lines = append(lines, fmt.Sprintf("        '-%s[%s]'", f.short, f.desc))
			}
		}
		return strings.Join(lines, "\n")
	case "fish":
		var lines []string
		for _, f := range flagDocs {
			line := "complete -c portlens -l " + f.long
			if f.short != "" {
				line += " -s " + f.short
			}
			lines = append(lines, line+" -d '"+f.desc+"'")
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// runCompletePorts outputs "port:description" for all current listening sockets,
// powering dynamic shell autocompletion.
func runCompletePorts(ctx context.Context, stdout io.Writer) int {
	plat := platform.New()
	if plat.Ports == nil {
		return exitcode.Success
	}
	// A completion request must never hang the shell: bound the listener scan
	// even when the caller's context carries no deadline. On timeout the
	// error path below silently yields no completions.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	listeners, err := plat.Ports.Listeners(ctx)
	if err != nil {
		return exitcode.Success
	}

	seen := make(map[uint16]bool)
	for _, l := range listeners {
		if seen[l.Port] {
			continue
		}
		seen[l.Port] = true

		desc := l.Process
		if desc == "" {
			desc = detect.LookupService(l.Port)
		}
		if desc == "" {
			desc = string(l.Protocol)
		}
		// Sanitize description for shell completions (remove colons and quotes)
		desc = strings.ReplaceAll(desc, ":", " ")
		desc = strings.ReplaceAll(desc, "'", "")
		desc = strings.ReplaceAll(desc, "\"", "")
		fmt.Fprintf(stdout, "%d:%s\n", l.Port, desc)
	}
	return exitcode.Success
}

const bashCompletionScript = `# Bash completion for portlens
# Add to ~/.bashrc: eval "$(portlens completion bash)"

_portlens_completions() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    if [[ "$prev" == "completion" ]]; then
        COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
        return 0
    fi

    if [[ "$prev" == "config" ]]; then
        COMPREPLY=( $(compgen -W "list add remove show path" -- "$cur") )
        return 0
    fi

    if [[ "$cur" == -* ]]; then
        local flags="{{FLAGS}}"
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
    fi

    local subcmds="{{COMMANDS}}"
    local ports=$(portlens --_complete_ports 2>/dev/null | cut -d: -f1)
    COMPREPLY=( $(compgen -W "$subcmds $ports" -- "$cur") )
}

complete -F _portlens_completions portlens
`

const zshCompletionScript = `#compdef portlens
# Zsh completion for portlens
# Add to ~/.zshrc: eval "$(portlens completion zsh)"

_portlens_ports() {
    local -a ports
    local IFS=$'\n'
    for line in $(portlens --_complete_ports 2>/dev/null); do
        ports+=("${line%%:*}:${line#*:}")
    done
    _describe -t ports 'listening ports' ports
}

_portlens() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    local -a subcommands
    subcommands=(
{{COMMANDS}}
    )

    local -a flags
    flags=(
{{FLAGS}}
    )

    _arguments -C \
        '1: :->cmd' \
        '*:: :->args' && return 0

    case $state in
        cmd)
            _portlens_ports
            _describe -t subcommands 'subcommands' subcommands
            _values 'flags' $flags
            ;;
        args)
            case $line[1] in
                config)
                    _values 'config actions' 'list' 'add' 'remove' 'show' 'path'
                    ;;
                completion)
                    _values 'shell' 'bash' 'zsh' 'fish'
                    ;;
                *)
                    _portlens_ports
                    ;;
            esac
            ;;
    esac
}

_portlens "$@"
`

const fishCompletionScript = `# Fish completion for portlens
# Add to ~/.config/fish/config.fish: portlens completion fish | source

function __portlens_ports
    portlens --_complete_ports 2>/dev/null | string replace -r '^([^:]+):(.*)$' '$1\t$2'
end

complete -c portlens -f
complete -c portlens -n '__fish_use_subcommand' -a '(__portlens_ports)' -d 'Listening port'
{{COMMANDS}}
{{FLAGS}}`
