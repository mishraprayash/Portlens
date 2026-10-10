package render

import (
	"fmt"
	"strings"

	"github.com/portlens/portlens/internal/model"
)

// procLabel returns a short, readable label for a process in the tree view.
func procLabel(p *model.ProcessInfo) string {
	if p == nil {
		return "?"
	}
	if p.Command != "" && !isGenericName(p.Name) {
		return p.Command
	}
	if p.Command != "" {
		return p.Command
	}
	return p.Name
}

func isGenericName(name string) bool {
	switch strings.ToLower(name) {
	case "node", "python", "python3", "ruby", "java", "sh", "bash", "zsh":
		return true
	}
	return false
}

// treeNode is a rendering-oriented process tree node.
type treeNode struct {
	info     *model.ProcessInfo
	children []*treeNode
	isTarget bool
}

// renderProcessTree builds the complete ancestor + descendant hierarchy and
// renders it, highlighting the process that owns the target port.
func (r *Renderer) renderProcessTree(report *model.Report) string {
	if report.Process == nil {
		return ""
	}
	root := buildTree(report)
	var lines []string
	r.walkTree(root, "", true, true, &lines)
	return strings.Join(lines, "\n")
}

func buildTree(report *model.Report) *treeNode {
	var root, cur *treeNode
	for _, a := range report.Ancestors {
		node := &treeNode{info: a}
		if report.Process != nil && a.PID == report.Process.PID {
			node.isTarget = true
		}
		if root == nil {
			root, cur = node, node
			continue
		}
		cur.children = append(cur.children, node)
		cur = node
	}
	if root == nil && report.Process != nil {
		root = &treeNode{info: report.Process, isTarget: true}
		cur = root
	}
	if cur != nil && report.Descendants != nil {
		attachDescendants(cur, report.Descendants)
	}
	return root
}

func attachDescendants(parent *treeNode, tree *model.ProcessTree) {
	if tree == nil {
		return
	}
	for _, c := range tree.Children {
		node := &treeNode{info: &c.Process}
		attachDescendants(node, c)
		parent.children = append(parent.children, node)
	}
}

func (r *Renderer) walkTree(node *treeNode, prefix string, isRoot, isLast bool, lines *[]string) {
	connector := ""
	switch {
	case isRoot:
		connector = ""
	case isLast:
		connector = "└── "
	default:
		connector = "├── "
	}

	label := procLabel(node.info)
	if node.isTarget {
		label = r.cyan(r.bold(label)) + r.dim(fmt.Sprintf("  (pid %d)  ← owns this port", node.info.PID))
	}
	*lines = append(*lines, prefix+connector+label)

	childPrefix := prefix
	switch {
	case isRoot:
		childPrefix = ""
	case isLast:
		childPrefix = prefix + "    "
	default:
		childPrefix = prefix + "│   "
	}

	for i, c := range node.children {
		r.walkTree(c, childPrefix, false, i == len(node.children)-1, lines)
	}
}
