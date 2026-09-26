package main

import (
	"fmt"
	"strings"
)

// ---- 命令行词法/语法 ----

type opType int

const (
	opStart opType = iota
	opPipe
	opAnd
	opOr
	opSemi
)

type pipeline struct {
	op   opType // 与上一个 pipeline 的连接方式（首个为 opStart）
	cmds [][]string
}

type tok struct {
	word bool
	val  string
}

// tokenize 切分命令行：支持 "..." 与 '...' 引号、反斜杠转义、|  ||  &&  ;
func tokenize(line string) ([]tok, error) {
	var toks []tok
	rs := []rune(line)
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			toks = append(toks, tok{word: true, val: string(cur)})
			cur = cur[:0]
		}
	}
	i := 0
	n := len(rs)
	for i < n {
		c := rs[i]
		switch c {
		case ' ', '\t':
			flush()
			i++
		case '\\':
			i++
			if i < n {
				cur = append(cur, rs[i])
				i++
			}
		case '"', '\'':
			q := c
			i++
			closed := false
			for i < n {
				if rs[i] == q {
					closed = true
					i++
					break
				}
				if rs[i] == '\\' && i+1 < n {
					cur = append(cur, rs[i+1])
					i += 2
					continue
				}
				cur = append(cur, rs[i])
				i++
			}
			if !closed {
				return nil, fmt.Errorf("引号未闭合")
			}
		case '|', '&', ';':
			flush()
			if c == '|' {
				if i+1 < n && rs[i+1] == '|' {
					toks = append(toks, tok{val: "||"})
					i += 2
				} else {
					toks = append(toks, tok{val: "|"})
					i++
				}
			} else if c == '&' {
				if i+1 < n && rs[i+1] == '&' {
					toks = append(toks, tok{val: "&&"})
					i += 2
				} else {
					toks = append(toks, tok{val: "&"})
					i++
				}
			} else {
				toks = append(toks, tok{val: ";"})
				i++
			}
		default:
			cur = append(cur, c)
			i++
		}
	}
	flush()
	return toks, nil
}

// parse 把 token 组装成用 ; / && / || 连接的 pipeline 序列，每个 pipeline 内含 | 分隔的命令
func parse(line string) ([]pipeline, error) {
	toks, err := tokenize(line)
	if err != nil {
		return nil, err
	}
	var pipes []pipeline
	curOp := opStart
	var curCmd []string
	var curPipe []string
	finalizeCmd := func() {
		if len(curCmd) > 0 {
			curPipe = append(curPipe, strings.Join(curCmd, "\x00"))
			curCmd = curCmd[:0]
		}
	}
	finalizePipe := func(op opType) error {
		finalizeCmd()
		if len(curPipe) == 0 {
			return fmt.Errorf("语法错误：运算符后缺少命令")
		}
		cmds := make([][]string, len(curPipe))
		for i, c := range curPipe {
			parts := strings.Split(c, "\x00")
			cmds[i] = parts
		}
		pipes = append(pipes, pipeline{op: curOp, cmds: cmds})
		curPipe = curPipe[:0]
		curOp = op
		return nil
	}
	for _, t := range toks {
		if t.word {
			curCmd = append(curCmd, t.val)
			continue
		}
		switch t.val {
		case "|":
			finalizeCmd()
			if len(curPipe) == 0 {
				return nil, fmt.Errorf("语法错误：| 前缺少命令")
			}
		case "&&", "||", ";":
			if err := finalizePipe(opSemi); err != nil {
				return nil, err
			}
			if t.val == "&&" {
				pipes[len(pipes)-1].op = opAnd
			} else if t.val == "||" {
				pipes[len(pipes)-1].op = opOr
			}
		case "&":
			return nil, fmt.Errorf("不支持单独的 &，请用 &&")
		}
	}
	if err := finalizePipe(opStart); err != nil {
		return nil, err
	}
	return pipes, nil
}

// ---- 执行 ----

// executeLine 执行一整行命令，把输出交给 UI 打印，返回退出码
func executeLine(line string) int {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0
	}
	pipes, err := parse(line)
	if err != nil {
		cmdPrint(err.Error())
		return 2
	}
	exit := 0
	for i, p := range pipes {
		if i > 0 {
			op := p.op
			if op == opAnd && exit != 0 {
				break
			}
			if op == opOr && exit == 0 {
				continue
			}
		}
		out := runPipeline(p.cmds)
		exit = out.code
		if out.text != "" {
			cmdPrint(out.text)
		}
	}
	return exit
}

type cmdResult struct {
	text string
	code int
}

// runPipeline 经典管道：stdout 依次接入下一命令 stdin，返回最后一个命令的结果
func runPipeline(cmds [][]string) cmdResult {
	var feed []byte
	var last cmdResult
	for _, c := range cmds {
		if len(c) == 0 {
			return cmdResult{text: "空命令", code: 127}
		}
		last = runCmd(c, feed)
		feed = []byte(last.text)
	}
	return last
}

func runCmd(args []string, stdin []byte) cmdResult {
	if len(args) == 0 {
		return cmdResult{code: 127, text: "空命令"}
	}
	name := args[0]
	if h, ok := builtins[name]; ok {
		out, code := h(args[1:])
		return cmdResult{text: out, code: code}
	}
	if isUnimplemented(name) {
		return cmdResult{text: fmt.Sprintf("%s：功能尚未实现，敬请期待", name), code: 1}
	}
	// 只支持内置命令，不执行外部命令
	return cmdResult{
		text: fmt.Sprintf("未知命令 %q（用 help 查看可用命令）", name),
		code: 127,
	}
}

var unimplemented = map[string]bool{
	"grab": true, "place": true,
}

func isUnimplemented(name string) bool {
	if unimplemented[name] {
		return true
	}
	for k := range unimplemented {
		if strings.HasPrefix(name, k+" ") {
			return true
		}
	}
	return false
}
