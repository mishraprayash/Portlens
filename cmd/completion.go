package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/portlens/portlens/internal/detect"
	"github.com/portlens/portlens/internal/exitcode"
	"github.com/portlens/portlens/internal/platform"
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

	switch shell {
	case "bash":
		fmt.Fprint(stdout, bashCompletionScript)
		return exitcode.Success
	case "zsh":
		fmt.Fprint(stdout, zshCompletionScript)
		return exitcode.Success
	case "fish":
		fmt.Fprint(stdout, fishCompletionScript)
		return exitcode.Success
	default:
		fmt.Fprintf(stderr, "portlens completion: unsupported shell %q (supported: bash, zsh, fish)\n", shell)
		return exitcode.InvalidArguments
	}
}

// runCompletePorts outputs "port:description" for all current listening sockets,
// powering dynamic shell autocompletion.
func runCompletePorts(stdout io.Writer) int {
	plat := platform.New()
	if plat.Ports == nil {
		return exitcode.Success
	}
	listeners, err := plat.Ports.Listeners(context.Background())
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
        local flags="--verbose -v --probe -p --tree --connections --json -j --kill -k --restart -r --open -o --watch -w --notify --yes -y --force -f --no-color --no-docker --debug -d --help -h --version --all --tcp --sort --filter"
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
    fi

    local subcmds="list ls inspect kill restart open tree conn connections watch find next config completion"
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
        'list:List active listening ports'
        'ls:List active listening ports'
        'inspect:Inspect port(s) with process details and exposure'
        'kill:Gracefully terminate process on port(s)'
        'restart:Restart process if launch command is known'
        'open:Open service in default browser'
        'tree:Display process hierarchy'
        'conn:Show active network connections'
        'connections:Show active network connections'
        'watch:Live-monitor port states with desktop notifications'
        'find:Find ports by process name or PID'
        'next:Find lowest available/free port'
        'config:Manage named port groups (@name)'
        'completion:Generate shell autocompletion script'
    )

    local -a flags
    flags=(
        '--verbose[Full detailed report (-v)]'
        '-v[Full detailed report]'
        '--probe[Probe HTTP endpoint for status, title, & server]'
        '-p[Probe HTTP endpoint for status, title, & server]'
        '--tree[Show complete process hierarchy]'
        '--connections[Show network connections]'
        '--json[JSON output]'
        '-j[JSON output]'
        '--kill[Gracefully terminate the owning process]'
        '-k[Gracefully terminate the owning process]'
        '--restart[Restart the process if launch command is known]'
        '-r[Restart the process]'
        '--open[Open service in browser]'
        '-o[Open service in browser]'
        '--watch[Re-render every interval]'
        '-w[Re-render every interval]'
        '--notify[Desktop notification on state change]'
        '--yes[Skip confirmations]'
        '-y[Skip confirmations]'
        '--force[Force SIGKILL]'
        '-f[Force SIGKILL]'
        '--no-color[Plain text output]'
        '--no-docker[Disable container detection]'
        '--debug[Diagnostic debug logging]'
        '-d[Diagnostic debug logging]'
        '--help[Show help]'
        '-h[Show help]'
        '--version[Print version]'
        '--all[Act on all listening ports]'
        '--tcp[Only show TCP listeners]'
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
complete -c portlens -n '__fish_use_subcommand' -a 'list' -d 'List active listening ports'
complete -c portlens -n '__fish_use_subcommand' -a 'inspect' -d 'Inspect port details'
complete -c portlens -n '__fish_use_subcommand' -a 'kill' -d 'Terminate process on port'
complete -c portlens -n '__fish_use_subcommand' -a 'restart' -d 'Restart process'
complete -c portlens -n '__fish_use_subcommand' -a 'open' -d 'Open in browser'
complete -c portlens -n '__fish_use_subcommand' -a 'tree' -d 'Show process hierarchy'
complete -c portlens -n '__fish_use_subcommand' -a 'conn' -d 'Show network connections'
complete -c portlens -n '__fish_use_subcommand' -a 'watch' -d 'Live monitor ports'
complete -c portlens -n '__fish_use_subcommand' -a 'find' -d 'Find ports by name or PID'
complete -c portlens -n '__fish_use_subcommand' -a 'next' -d 'Find next available port'
complete -c portlens -n '__fish_use_subcommand' -a 'config' -d 'Manage named port groups'
complete -c portlens -n '__fish_use_subcommand' -a 'completion' -d 'Generate completion script'
complete -c portlens -l verbose -s v -d 'Full detailed report'
complete -c portlens -l probe -s p -d 'Probe HTTP endpoint'
complete -c portlens -l tree -d 'Show process hierarchy'
complete -c portlens -l connections -d 'Show network connections'
complete -c portlens -l json -s j -d 'JSON output'
complete -c portlens -l kill -s k -d 'Gracefully terminate owning process'
complete -c portlens -l restart -s r -d 'Restart process'
complete -c portlens -l open -s o -d 'Open in browser'
complete -c portlens -l watch -s w -d 'Watch mode'
complete -c portlens -l notify -d 'Desktop notification'
complete -c portlens -l yes -s y -d 'Skip confirmations'
complete -c portlens -l force -s f -d 'Force SIGKILL'
complete -c portlens -l no-color -d 'Plain output'
complete -c portlens -l no-docker -d 'Disable container detection'
complete -c portlens -l help -s h -d 'Show help'
complete -c portlens -l version -d 'Show version'
`
