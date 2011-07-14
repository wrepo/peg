// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package peg

import (
	"fmt"
	"container/list"
	"os"
	"io"
)

type Type uint8

const (
	TypeUnknown Type = iota
	TypeRule
	TypeVariable
	TypeName
	TypeDot
	TypeCharacter
	TypeString
	TypeClass
	TypePredicate
	TypeCommit
	TypeBegin
	TypeEnd
	TypeAction
	TypeAlternate
	TypeUnorderedAlternate
	TypeSequence
	TypePeekFor
	TypePeekNot
	TypeQuery
	TypeStar
	TypePlus
	TypeNil
	TypeLast
)

func (t Type) GetType() Type {
	return t
}

type Node interface {
	fmt.Stringer
	GetType() Type
}

/* Used to represent TypeRule*/
type Rule interface {
	Node
	GetId() int
	GetExpression() Node
	SetExpression(e Node)
}

type rule struct {
	name       string
	id         int
	expression Node
	hasActions bool
	variables  map[string]*variable
}

func (r *rule) GetType() Type {
	return TypeRule
}

func (r *rule) GetId() int {
	return r.id
}

func (r *rule) GetExpression() Node {
	if r.expression == nil {
		return nilNode
	}
	return r.expression
}

func (r *rule) SetExpression(e Node) {
	r.expression = e
}

func (r *rule) String() string {
	return r.name
}

func (r *rule) goString() string {
	b := []byte(r.String())
	for i := 0; i < len(b); i++ {
		if b[i] == '-' {
			b[i] = '_'
		}
	}
	return string(b)
}

type variable struct {
	name   string
	offset int
}

/* Used to represent TypeName */
type Name interface {
	Node
}

type name struct {
	Type
	string
	varp *variable
}

func (t *name) String() string {
	return t.string
}

/* Used to represent TypeDot, TypeCharacter, TypeString, TypeClass, TypePredicate, and TypeNil. */
type Token interface {
	Node
	GetClass() *characterClass
}

type token struct {
	Type
	string
	class *characterClass
}

func (t *token) GetClass() *characterClass {
	return t.class
}

func (t *token) String() string {
	return t.string
}

var nilNode = &token{Type: TypeNil, string: "<nil>"}

/* Used to represent TypeAction. */
type Action interface {
	Node
	GetId() int
	GetRule() string
}

type action struct {
	text string
	id   int
	rule *rule
}

func (a *action) GetType() Type {
	return TypeAction
}

func (a *action) String() string {
	return a.text
}

func (a *action) Print(w io.Writer) {
	vmap := a.rule.variables
	ind := "\n\t\t\t"
	off := 0
	for _, v := range vmap {
		off--
		v.offset = off
		fmt.Fprintf(w, ind+"%s := yyval[yyp%d]", v.name, v.offset)
	}
	fmt.Fprintf(w, ind+"%v", a)
	for _, v := range vmap {
		fmt.Fprintf(w, ind+"yyval[yyp%d] = %s", v.offset, v.name)
	}
}

func (a *action) GetId() int {
	return a.id
}

func (a *action) GetRule() string {
	return a.rule.String()
}

/* Used to represent a TypeAlternate, TypeSequence, TypePeekFor, TypePeekNot, TypeQuery, TypeStar, or TypePlus */

type List interface {
	Node
	SetType(t Type)

	Init() *list.List
	Front() *list.Element
	PushBack(value interface{}) *list.Element
	Len() int
}

type nodeList struct {
	Type
	list.List
}

func (l *nodeList) SetType(t Type) {
	l.Type = t
}

func (l *nodeList) String() string {
	i := l.List.Front()
	s := "(" + i.Value.(fmt.Stringer).String()
	for i = i.Next(); i != nil; i = i.Next() {
		s += " / " + i.Value.(fmt.Stringer).String()
	}
	return s + ")"
}

/* Used to represent character classes. */
type characterClass [32]uint8

func (c *characterClass) copy() (class *characterClass) {
	class = new(characterClass)
	copy(class[0:], c[0:])
	return
}
func (c *characterClass) add(character uint8)      { c[character>>3] |= (1 << (character & 7)) }
func (c *characterClass) has(character uint8) bool { return c[character>>3]&(1<<(character&7)) != 0 }
func (c *characterClass) complement() {
	for i := range *c {
		c[i] = ^c[i]
	}
}
func (c *characterClass) union(class *characterClass) {
	for index, value := range *class {
		c[index] |= value
	}
}
func (c *characterClass) intersection(class *characterClass) {
	for index, value := range *class {
		c[index] &= value
	}
}
func (c *characterClass) len() (length int) {
	for character := 0; character < 256; character++ {
		if c.has(uint8(character)) {
			length++
		}
	}
	return
}
func (c *characterClass) String() (class string) {
	escape := func(c uint8) string {
		s := ""
		switch uint8(c) {
		case '\a':
			s = `\a` /* bel */
		case '\b':
			s = `\b` /* bs */
		case '\f':
			s = `\f` /* ff */
		case '\n':
			s = `\n` /* nl */
		case '\r':
			s = `\r` /* cr */
		case '\t':
			s = `\t` /* ht */
		case '\v':
			s = `\v` /* vt */
		case '\'':
			s = `\'` /* ' */
		case '"':
			s = `\"` /* " */
		case '[':
			s = `\[` /* [ */
		case ']':
			s = `\]` /* ] */
		case '\\':
			s = `\\` /* \ */
		case '-':
			s = `\-` /* - */
		default:
			s = fmt.Sprintf("%c", c)
		}
		return s
	}
	class = ""
	l := 0
	for character := 0; character < 256; character++ {
		if c.has(uint8(character)) {
			if l == 0 {
				class += escape(uint8(character))
			}
			l++
		} else {
			if l == 2 {
				class += escape(uint8(character - 1))
			} else if l > 2 {
				class += "-" + escape(uint8(character-1))
			}
			l = 0
		}
	}
	if l >= 2 {
		class += "-" + escape(255)
	}
	return
}

/* A tree data structure into which a PEG can be parsed. */
type Tree struct {
	rules      map[string]*rule
	rulesCount map[string]uint
	ruleId     int
	varp       *variable
	headers    []string
	trailers   []string
	list.List
	Actions         []*action
	classes         map[string]*characterClass
	defines         map[string]string
	switchExcl      map[string]bool
	stack           [1024]Node
	top             int
	inline, _switch bool
}

func New(inline, _switch bool) *Tree {
	return &Tree{rules: make(map[string]*rule),
		rulesCount: make(map[string]uint),
		classes:    make(map[string]*characterClass),
		defines: map[string]string{
			"package":   "",
			"Peg":       "yyParser",
			"userstate": "",
			"yystype":   "yyStype",
		},
		inline:  inline,
		_switch: _switch}
}

func (t *Tree) push(n Node) {
	t.top++
	t.stack[t.top] = n
}

func (t *Tree) pop() Node {
	n := t.stack[t.top]
	t.top--
	return n
}

func (t *Tree) currentRule() *rule {
	return t.stack[1].(*rule)
}

func (t *Tree) AddRule(name string) {
	t.push(&rule{name: name, id: t.ruleId})
	t.ruleId++
}

func (t *Tree) AddExpression() {
	expression := t.pop()
	rule := t.pop().(Rule)
	rule.SetExpression(expression)
	t.PushBack(rule)
}

func (t *Tree) AddHeader(text string) {
	t.headers = append(t.headers, text)
}

func (t *Tree) AddTrailer(text string) {
	t.trailers = append(t.trailers, text)
}

func (t *Tree) AddVariable(text string) {
	var v *variable

	r := t.currentRule()
	if r.variables == nil {
		r.variables = make(map[string]*variable)
	}
	if v = r.variables[text]; v == nil {
		v = &variable{name: text}
	}
	r.variables[text] = v
	t.varp = v
}

func (t *Tree) AddName(text string) {
	t.rules[text] = &rule{}
	t.push(&name{Type: TypeName, string: text, varp: t.varp})
	t.varp = nil
}

var dot *token = &token{Type: TypeDot, string: "."}

func (t *Tree) AddDot() { t.push(dot) }
func (t *Tree) AddString(text string) {
	length := len(text)
	if (length == 1) || ((length == 2) && (text[0] == '\\')) {
		t.push(&token{Type: TypeCharacter, string: text})
	} else {
		t.push(&token{Type: TypeString, string: text})
	}
}
func (t *Tree) AddClass(text string) {
	t.push(&token{Type: TypeClass, string: text})
	if c, ok := t.classes[text]; !ok {
		c = new(characterClass)
		t.classes[text] = c
		inverse := false
		if text[0] == '^' {
			inverse = true
			text = text[1:]
		}
		var last uint8
		hasLast := false
		for i := 0; i < (len(text) - 1); i++ {
			switch {
			case (text[i] == '-') && hasLast:
				i++
				for j := last; j <= text[i]; j++ {
					c.add(j)
				}
				hasLast = false
			case (text[i] == '\\'):
				i++
				last, hasLast = text[i], true
				switch last {
				case 'a':
					last = '\a' /* bel */
				case 'b':
					last = '\b' /* bs */
				case 'f':
					last = '\f' /* ff */
				case 'n':
					last = '\n' /* nl */
				case 'r':
					last = '\r' /* cr */
				case 't':
					last = '\t' /* ht */
				case 'v':
					last = '\v' /* vt */
				}
				c.add(last)
			default:
				last, hasLast = text[i], true
				c.add(last)
			}
		}
		c.add(text[len(text)-1])
		if inverse {
			c.complement()
		}
	}
}
func (t *Tree) AddPredicate(text string) { t.push(&token{Type: TypePredicate, string: text}) }

var commit *token = &token{Type: TypeCommit, string: "commit"}

func (t *Tree) AddCommit() { t.push(commit) }

var begin *token = &token{Type: TypeBegin, string: "<"}

func (t *Tree) AddBegin() { t.push(begin) }

var end *token = &token{Type: TypeEnd, string: ">"}

func (t *Tree) AddEnd() { t.push(end) }
func (t *Tree) AddNil() { t.push(nilNode) }
func (t *Tree) AddAction(text string) {
	b := []byte(text)
	for i := 0; i < len(b)-1; i++ {
		if b[i] == '$' && b[i+1] == '$' {
			b[i], b[i+1] = 'y', 'y'
		}
	}
	a := &action{text: string(b), id: len(t.Actions), rule: t.currentRule()}
	t.currentRule().hasActions = true
	t.Actions = append(t.Actions, a)
	t.push(a)
}
func (t *Tree) Define(name, text string) {
	if _, ok := t.defines[name]; ok {
		t.defines[name] = text
	}
}
func (t *Tree) SwitchExclude(rule string) {
	if t.switchExcl == nil {
		t.switchExcl = make(map[string]bool, 16)
	}
	t.switchExcl[rule] = true
}

func (t *Tree) addList(listType Type) {
	a := t.pop()
	b := t.pop()
	var l List
	if b.GetType() == listType {
		l = b.(List)
	} else {
		l = &nodeList{Type: listType}
		l.PushBack(b)
	}
	l.PushBack(a)
	t.push(l)
}
func (t *Tree) AddAlternate() { t.addList(TypeAlternate) }
func (t *Tree) AddSequence() { t.addList(TypeSequence) }

func (t *Tree) addFix(fixType Type) {
	n := &nodeList{Type: fixType}
	n.PushBack(t.pop())
	t.push(n)
}
func (t *Tree) AddPeekFor()         { t.addFix(TypePeekFor) }
func (t *Tree) AddPeekNot()         { t.addFix(TypePeekNot) }
func (t *Tree) AddQuery()           { t.addFix(TypeQuery) }
func (t *Tree) AddStar()            { t.addFix(TypeStar) }
func (t *Tree) AddPlus()            { t.addFix(TypePlus) }

func join(tasks []func()) {
	length := len(tasks)
	done := make(chan int, length)
	for _, task := range tasks {
		go func(task func()) { task(); done <- 1 }(task)
	}
	for d := <-done; d < length; d += <-done {
	}
}

var emptyString = new(characterClass)

func (t *Tree) Compile(file string) {
	counts := [TypeLast]uint{}
	nvar := 0

	for element := t.Front(); element != nil; element = element.Next() {
		node := element.Value.(Node)
		switch node.GetType() {
		case TypeRule:
			rule := node.(*rule)
			t.rules[rule.String()] = rule
			nvar += len(rule.variables)
		}
	}
	for name, r := range t.rules {
		if r.name == "" {
			r := &rule{name: name, id: t.ruleId}
			t.ruleId++
			t.rules[name] = r
			t.PushBack(r)
		}
	}

	join([]func(){
		func() {
			var countTypes func(node Node)
			countTypes = func(node Node) {
				t := node.GetType()
				counts[t]++
				switch t {
				case TypeRule:
					countTypes(node.(Rule).GetExpression())
				case TypeAlternate, TypeUnorderedAlternate, TypeSequence:
					for element := node.(List).Front(); element != nil; element = element.Next() {
						countTypes(element.Value.(Node))
					}
				case TypePeekFor, TypePeekNot, TypeQuery, TypeStar, TypePlus:
					countTypes(node.(List).Front().Value.(Node))
				}
			}
			for _, rule := range t.rules {
				countTypes(rule)
			}
		},
		func() {
			var countRules func(node Node)
			ruleReached := make([]bool, len(t.rules))
			countRules = func(node Node) {
				switch node.GetType() {
				case TypeRule:
					rule := node.(Rule)
					name, id := rule.String(), rule.GetId()
					if count, ok := t.rulesCount[name]; ok {
						t.rulesCount[name] = count + 1
					} else {
						t.rulesCount[name] = 1
					}
					if ruleReached[id] {
						return
					}
					ruleReached[id] = true
					countRules(rule.GetExpression())
				case TypeName:
					countRules(t.rules[node.String()])
				case TypeAlternate, TypeUnorderedAlternate, TypeSequence:
					for element := node.(List).Front(); element != nil; element = element.Next() {
						countRules(element.Value.(Node))
					}
				case TypePeekFor, TypePeekNot, TypeQuery, TypeStar, TypePlus:
					countRules(node.(List).Front().Value.(Node))
				}
			}
			for element := t.Front(); element != nil; element = element.Next() {
				node := element.Value.(Node)
				if node.GetType() == TypeRule {
					countRules(node.(*rule))
					break
				}
			}
		},
		func() {
			var checkRecursion func(node Node) bool
			ruleReached := make([]bool, len(t.rules))
			checkRecursion = func(node Node) bool {
				switch node.GetType() {
				case TypeRule:
					rule := node.(Rule)
					id := rule.GetId()
					if ruleReached[id] {
						fmt.Fprintf(os.Stderr, "possible infinite left recursion in rule '%v'\n", node)
						return false
					}
					ruleReached[id] = true
					consumes := checkRecursion(rule.GetExpression())
					ruleReached[id] = false
					return consumes
				case TypeAlternate:
					for element := node.(List).Front(); element != nil; element = element.Next() {
						if !checkRecursion(element.Value.(Node)) {
							return false
						}
					}
					return true
				case TypeSequence:
					for element := node.(List).Front(); element != nil; element = element.Next() {
						if checkRecursion(element.Value.(Node)) {
							return true
						}
					}
				case TypeName:
					return checkRecursion(t.rules[node.String()])
				case TypePlus:
					return checkRecursion(node.(List).Front().Value.(Node))
				case TypeCharacter, TypeString:
					return len(node.String()) > 0
				case TypeDot, TypeClass:
					return true
				}
				return false
			}
			for _, rule := range t.rules {
				checkRecursion(rule)
			}
		}})

	if t._switch {
		var optimizeAlternates func(node Node) (consumes, eof, peek bool, class *characterClass)
		cache := make([]struct {
			reached, consumes, eof, peek bool
			class                        *characterClass
		}, len(t.rules))
		optimizeAlternates = func(node Node) (consumes, eof, peek bool, class *characterClass) {
			switch node.GetType() {
			case TypeRule:
				rule := node.(Rule)
				if t.switchExcl != nil && t.switchExcl[rule.String()] {
					return
				}
				cache := &cache[rule.GetId()]
				if cache.reached {
					consumes, eof, peek, class = cache.consumes, cache.eof, cache.peek, cache.class
					return
				}
				cache.reached = true
				consumes, eof, peek, class = optimizeAlternates(rule.GetExpression())
				cache.consumes, cache.eof, cache.peek, cache.class = consumes, eof, peek, class
			case TypeName:
				consumes, eof, peek, class = optimizeAlternates(t.rules[node.String()])
			case TypeDot:
				consumes, class = true, new(characterClass)
				for index, _ := range *class {
					class[index] = 0xff
				}
			case TypeString, TypeCharacter:
				if node.String() == "" {
					consumes, class = true, emptyString
					return
				}
				consumes, class = true, new(characterClass)
				b := node.String()[0]
				if b == '\\' {
					b = node.String()[1]
					switch b {
					case 'a':
						b = '\a' /* bel */
					case 'b':
						b = '\b' /* bs */
					case 'f':
						b = '\f' /* ff */
					case 'n':
						b = '\n' /* nl */
					case 'r':
						b = '\r' /* cr */
					case 't':
						b = '\t' /* ht */
					case 'v':
						b = '\v' /* vt */
					}
				}
				class.add(b)
			case TypeClass:
				consumes, class = true, t.classes[node.String()]
			case TypeAlternate:
				consumes, peek, class = true, true, new(characterClass)
				alternate := node.(List)
				mconsumes, meof, mpeek, properties, c :=
					consumes, eof, peek, make([]struct {
						intersects bool
						class      *characterClass
					}, alternate.Len()), 0
				for element := alternate.Front(); element != nil; element = element.Next() {
					mconsumes, meof, mpeek, properties[c].class = optimizeAlternates(element.Value.(Node))
					if c+1 == len(properties) && properties[c].class == emptyString {
						properties = properties[:c]
						break
					}
					consumes, eof, peek = consumes && mconsumes, eof || meof, peek && mpeek
					if properties[c].class != nil {
						class.union(properties[c].class)
					}
					c++
				}
				if eof {
					break
				}
				intersections := 2
			compare:
				for ai, a := range properties[0 : len(properties)-1] {
					for _, b := range properties[ai+1:] {
						for i, v := range *a.class {
							if (b.class[i] & v) != 0 {
								intersections++
								properties[ai].intersects = true
								continue compare
							}
						}
					}
				}
				if intersections < len(properties) {
					c, unordered, ordered, max :=
						0, &nodeList{Type: TypeUnorderedAlternate}, &nodeList{Type: TypeAlternate}, 0
					for element := alternate.Front(); element != nil; element = element.Next() {
						if properties[c].intersects {
							ordered.PushBack(element.Value)
						} else {
							class := &token{Type: TypeClass, string: properties[c].class.String(), class: properties[c].class}

							sequence, predicate, length := 
								&nodeList{Type: TypeSequence}, &nodeList{Type: TypePeekFor}, properties[c].class.len()
							predicate.PushBack(class)
							sequence.PushBack(predicate)
							sequence.PushBack(element.Value)

							if element.Value.(Node).GetType() == TypeNil {
								unordered.PushBack(sequence)
							} else if length > max {
								unordered.PushBack(sequence)
								max = length
							} else {
								unordered.PushFront(sequence)
							}
						}
						c++
					}
					alternate.Init()
					if ordered.Len() == 0 {
						alternate.SetType(TypeUnorderedAlternate)
						for element := unordered.Front(); element != nil; element = element.Next() {
							alternate.PushBack(element.Value)
						}
					} else {
						for element := ordered.Front(); element != nil; element = element.Next() {
							alternate.PushBack(element.Value)
						}
						alternate.PushBack(unordered)
					}
				}
			case TypeSequence:
				sequence := node.(List)
				meof, classes, c, element :=
					eof, make([]struct {
						peek  bool
						class *characterClass
					}, sequence.Len()), 0, sequence.Front()
				for ; !consumes && element != nil; element, c = element.Next(), c + 1 {
					consumes, meof, classes[c].peek, classes[c].class = optimizeAlternates(element.Value.(Node))
					eof, peek = eof || meof, peek || classes[c].peek
				}
				eof, peek, class = !consumes && eof, !consumes && peek, new(characterClass)
				for c--; c >= 0; c-- {
					if classes[c].class != nil {
						if classes[c].peek {
							class.intersection(classes[c].class)
						} else {
							class.union(classes[c].class)
						}
					}
				}
				for ; element != nil; element = element.Next() {
					optimizeAlternates(element.Value.(Node))
				}
			case TypePeekNot:
				peek = true
				_, eof, _, class = optimizeAlternates(node.(List).Front().Value.(Node))
				eof = !eof
				class = class.copy()
				class.complement()
			case TypePeekFor:
				peek = true
				fallthrough
			case TypeQuery, TypeStar:
				_, eof, _, class = optimizeAlternates(node.(List).Front().Value.(Node))
			case TypePlus:
				consumes, eof, peek, class = optimizeAlternates(node.(List).Front().Value.(Node))
			case TypeAction, TypeNil:
				class = new(characterClass)
 			}
			return
		}
		for element := t.Front(); element != nil; element = element.Next() {
			node := element.Value.(Node)
			if node.GetType() == TypeRule {
				optimizeAlternates(node.(*rule))
				break
			}
		}
	}

	out, error := os.Create(file)
	if error != nil {
		fmt.Printf("%v: %v\n", file, error)
		return
	}
	defer out.Close()

	w := newWriter(out)
	print := func(format string, a ...interface{}) {
		if !w.dryRun {
			fmt.Fprintf(w, format, a...)
		}
	}

	for _, s := range t.headers {
		print("%s", s)
	}

	if p := t.defines["package"]; p != "" {
		print(`package %v

import (
	"fmt"
	"peg"
)`, p)
	}
	print("\nconst (\n")
	for el := t.Front(); el != nil; el = el.Next() {
		node := el.Value.(Node)
		if node.GetType() != TypeRule {
			continue
		}
		r := node.(*rule)
		if r.GetId() == 0 {
			print("\trule%s\t= iota\n", r.goString())
		} else {
			print("\trule%s\n", r.goString())
		}
	}
	pegname := t.defines["Peg"]
	print(
		`)

type %v struct {%v
	Buffer string
	Min, Max int
	rules [%d]func() bool
	ResetBuffer	func(string) string
}

func (p *%v) Parse(ruleId int) bool {
	if p.rules[ruleId]() {
		return true
	}
	return false
}
func (p *%v) PrintError() {
	line := 1
	character := 0
	for i, c := range p.Buffer[0:] {
		if c == '\n' {
			line++
			character = 0
		} else {
			character++
		}
		if i == p.Min {
			if p.Min != p.Max {
				fmt.Printf("parse error after line %%v character %%v\n", line, character)
			} else {
				break
			}
		} else if i == p.Max {
			break
		}
	}
	fmt.Printf("parse error: unexpected ")
	if p.Max >= len(p.Buffer) {
		fmt.Printf("end of file found\n")
	} else {
		fmt.Printf("'%%c' at line %%v character %%v\n", p.Buffer[p.Max], line, character)
	}
}
func (p *%v) Init() {
	var position int`,
		pegname, t.defines["userstate"], len(t.rules), pegname, pegname, pegname)

	if nvar > 0 {
		yystype := t.defines["yystype"]
		print(
			`
`+`	var yyp int
`+`	var yy %s
`+`	var yyval = make([]%s, 200)
`,
			yystype, yystype)
	}

	hasActions := t.Actions != nil
	if hasActions {
		bits := 0
		for length := len(t.Actions); length != 0; length >>= 1 {
			bits++
		}
		switch {
		case bits < 8:
			bits = 8
		case bits < 16:
			bits = 16
		case bits < 32:
			bits = 32
		case bits < 64:
			bits = 64
		}
		print("\n\tactions := [...]func(string, int){")
		for _, a := range t.Actions {
			w.lnPrint("/* %v %v */", a.GetId(), a.GetRule())
			w.lnPrint("func(yytext string, _ int) {")
			a.Print(w)
			w.lnPrint("},")
		}
		if nvar > 0 {
			nact := len(t.Actions)
			print(`
`+`		/* %d yyPush */
`+`		func(_ string, count int) {
`+`			yyp += count
`+`			if yyp >= len(yyval) {
`+`				s := make([]%s, cap(yyval)+200)
`+`				copy(s, yyval)
`+`				yyval = s
`+`			}
`+`		},
`+`		/* %d yyPop */
`+`		func(_ string, count int) {
`+`			yyp -= count
`+`		},
`+`		/* %d yySet */
`+`		func(_ string, count int) {
`+`			yyval[yyp+count] = yy
`+`		},
`+`	}
`+`	const (
`+`		yyPush = %d+iota
`+`		yyPop
`+`		yySet
`+`	)
`,
				nact, t.defines["yystype"], nact+1, nact+2, nact)
		} else {
			print("\t}\n")
		}

		print(
			`
`+`	var thunkPosition, begin, end int
`+`	thunks := make([]struct {action uint%d; begin, end int}, 32)
`+`	doarg := func(action uint%d, arg int) {
`+`		if thunkPosition == len(thunks) {
`+`			newThunks := make([]struct {action uint%d; begin, end int}, 2 * len(thunks))
`+`			copy(newThunks, thunks)
`+`			thunks = newThunks
`+`		}
`+`		thunks[thunkPosition].action = action
`+`		if arg != 0 {
`+`			thunks[thunkPosition].begin = arg // use begin to store an argument
`+`		} else {
`+`			thunks[thunkPosition].begin = begin
`+`		}
`+`		thunks[thunkPosition].end = end
`+`		thunkPosition++
`+`	}
`+`	do := func(action uint%d) {
`+`		doarg(action, 0)
`+`	}`, bits, bits, bits, bits)

		print(
			`
`+`	p.ResetBuffer = func(s string) (old string) {
`+`		if p.Max < len(p.Buffer) {
`+`			old = p.Buffer[p.Max:]
`+`		}
`+`		p.Buffer = s
`+`		thunkPosition = 0
`+`		position = 0
`+`		p.Min = 0
`+`		p.Max = 0
`+`		return
`+`	}
`)
		if counts[TypeCommit] > 0 {
			print(
				`
`+`	commit := func(thunkPosition0 int) bool {
`+`		if thunkPosition0 == 0 {
`+`			for i := 0; i < thunkPosition; i++ {
`+`				b := thunks[i].begin
`+`				e := thunks[i].end
`+`				s := ""
`+`				if b>=0 && e<=len(p.Buffer) && b<=e {
`+`					s = p.Buffer[b:e]
`+`				}
`+`				magic := b
`+`				actions[thunks[i].action](s, magic)
`+`			}
`+`			p.Min = position
`+`			thunkPosition = 0
`+`			return true
`+`		}
`+`		return false
`+`	}`)
		}
		w.hasCommit = true
	}

	if counts[TypeDot] > 0 {
		print(
			`
`+`	matchDot := func() bool {
`+`		if position < len(p.Buffer) {
`+`			position++
`+`			return true
`+`		} else if position >= p.Max {
`+`			p.Max = position
`+`		}
`+`		return false
`+`	}
`+`	peekDot := func() bool {
`+`		return position < len(p.Buffer)
`+`	}
`+`	_ = peekDot
`)
	}
	if counts[TypeCharacter] > 0 {
		print(
			`
`+`	matchChar := func(c byte) bool {
`+`		if (position < len(p.Buffer)) && (p.Buffer[position] == c) {
`+`			position++
`+`			return true
`+`		} else if position >= p.Max {
`+`			p.Max = position
`+`		}
`+`		return false
`+`	}
`+`	peekChar := func(c byte) bool {
`+`		return position < len(p.Buffer) && p.Buffer[position] == c
`+`	}
`+`	_ = peekChar
`)
	}
	if counts[TypeString] > 0 {
		print(
			`
`+`	matchString := func(s string) bool {
`+`		length := len(s)
`+`		next := position + length
`+`		if (next <= len(p.Buffer)) && (p.Buffer[position:next] == s) {
`+`			position = next
`+`			return true
`+`		} else if position >= p.Max {
`+`			p.Max = position
`+`		}
`+`		return false
`+`	}`)
	}

	classes := make(map[string]uint)
	if len(t.classes) != 0 {
		print("\n\tclasses := [...][32]uint8{\n")
		var index uint
		for className, classBitmap := range t.classes {
			classes[className] = index
			print("\t\t{")
			sep := ""
			for _, b := range *classBitmap {
				print("%s%d", sep, b)
				sep = ", "
			}
			print("},\n")
			index++
		}
		print(
			`	}
`+`	matchClass := func(class uint) bool {
`+`		if (position < len(p.Buffer)) &&
`+`			((classes[class][p.Buffer[position]>>3] & (1 << (p.Buffer[position] & 7))) != 0) {
`+`			position++
`+`			return true
`+`		} else if position >= p.Max {
`+`			p.Max = position
`+`		}
`+`		return false
`+`	}`)
	}

	var printRule func(node Node)
	var compile func(expression Node, ko *label) (chgFlags, chgFlags)
	printRule = func(node Node) {
		switch node.GetType() {
		case TypeRule:
			print("%v <- ", node)
			expression := node.(Rule).GetExpression()
			if expression != nilNode {
				printRule(expression)
			}
		case TypeDot:
			print(".")
		case TypeName:
			print("%v", node)
		case TypeCharacter,
			TypeString:
			print("'%v'", node)
		case TypeClass:
			print("[%v]", node)
		case TypePredicate:
			print("&{%v}", node)
		case TypeAction:
			print("{%v}", node)
		case TypeCommit:
			print("commit")
		case TypeBegin:
			print("<")
		case TypeEnd:
			print(">")
		case TypeAlternate:
			print("(")
			list := node.(List)
			element := list.Front()
			printRule(element.Value.(Node))
			for element = element.Next(); element != nil; element = element.Next() {
				print(" / ")
				printRule(element.Value.(Node))
			}
			print(")")
		case TypeUnorderedAlternate:
			print("(")
			element := node.(List).Front()
			printRule(element.Value.(Node))
			for element = element.Next(); element != nil; element = element.Next() {
				print(" | ")
				printRule(element.Value.(Node))
			}
			print(")")
		case TypeSequence:
			print("(")
			element := node.(List).Front()
			printRule(element.Value.(Node))
			for element = element.Next(); element != nil; element = element.Next() {
				print(" ")
				printRule(element.Value.(Node))
			}
			print(")")
		case TypePeekFor:
			print("&")
			printRule(node.(List).Front().Value.(Node))
		case TypePeekNot:
			print("!")
			printRule(node.(List).Front().Value.(Node))
		case TypeQuery:
			printRule(node.(List).Front().Value.(Node))
			print("?")
		case TypeStar:
			printRule(node.(List).Front().Value.(Node))
			print("*")
		case TypePlus:
			printRule(node.(List).Front().Value.(Node))
			print("+")
		default:
			fmt.Fprintf(os.Stderr, "illegal node type: %v\n", node.GetType())
		}
	}
	compileExpression := func(rule *rule, ko *label) (cko, cok chgFlags) {
		nvar := len(rule.variables)
		if nvar > 0 {
			w.lnPrint("doarg(yyPush, %d)", nvar)
		}
		cko, cok = compile(rule.GetExpression(), ko)
		if nvar > 0 {
			w.lnPrint("doarg(yyPop, %d)", nvar)
			cko.thPos = true
			cok.thPos = true
		}
		return
	}
	canCompilePeek := func(node Node, jumpIfTrue bool, label *label) bool {
		switch node.GetType() {
		case TypeDot:
			label.cJump(jumpIfTrue, "peekDot()")
		case TypeCharacter:
			label.cJump(jumpIfTrue, "peekChar('%v')", node)
		case TypePredicate:
			label.cJump(jumpIfTrue, "(%v)", node)
		default:
			return false
		}
		return true
	}
	compile = func(node Node, ko *label) (chgko, chgok chgFlags) {
		updateFlags := func(cko, cok chgFlags) (chgFlags, chgFlags) {
			if cko.pos { chgko.pos = true }
			if cko.thPos { chgko.thPos = true }
			if cok.pos { chgok.pos = true }
			if cok.thPos { chgok.thPos = true }
			return cko, cok
		}
		switch node.GetType() {
		case TypeRule:
			fmt.Fprintf(os.Stderr, "internal error #1 (%v)\n", node)
		case TypeDot:
			ko.cJump(false, "matchDot()")
			chgok.pos = true
		case TypeName:
			varp := node.(*name).varp
			name := node.String()
			rule := t.rules[name]
			if t.inline && t.rulesCount[name] == 1 {
				chgko, chgok = compileExpression(rule, ko)
			} else {
				ko.cJump(false, "p.rules[rule%s]()", rule.goString())
				if len(rule.variables) != 0 || rule.hasActions {
					chgok.thPos = true
				}
				chgok.pos = true	// safe guess
			}
			if varp != nil {
				w.lnPrint("doarg(yySet, %d)", varp.offset)
				chgok.thPos = true
			}
		case TypeCharacter:
			ko.cJump(false, "matchChar('%v')", node)
			chgok.pos = true
		case TypeString:
			s := node.String()
			if s == "" {
				ko.cJump(false, "peekDot()")
			} else {
				ko.cJump(false, "matchString(\"%s\")", s)
			}
			chgok.pos = true
		case TypeClass:
			ko.cJump(false, "matchClass(%d)", classes[node.String()])
			chgok.pos = true
		case TypePredicate:
			ko.cJump(false, "(%v)", node)
		case TypeAction:
			w.lnPrint("do(%d)", node.(Action).GetId())
			chgok.thPos = true
		case TypeCommit:
			ko.cJump(false, "(commit(thunkPosition0))")
			chgko.thPos = true
		case TypeBegin:
			if hasActions {
				w.lnPrint("begin = position")
			}
		case TypeEnd:
			if hasActions {
				w.lnPrint("end = position")
			}
		case TypeAlternate:
			list := node.(List)
			ok := w.newLabel()
			element := list.Front()
			if ok.unsafe() {
				w.begin()
				ok.save()
			}
			var next *label
			for element.Next() != nil {
				next = w.newLabel()
				cko, _ := updateFlags(compile(element.Value.(Node), next))
				ok.jump()
				if next.used {
					ok.lrestore(next, cko.pos, cko.thPos)
				}
				element = element.Next()
			}
			if next == nil || next.used {
				updateFlags(compile(element.Value.(Node), ko))
			}
			if ok.unsafe() {
				w.end()
			}
			if ok.used {
				ok.label()
			}
		case TypeUnorderedAlternate:
			list := node.(List)
			done, ok := ko, w.newLabel()
			w.begin()
			done.cJump(true, "position == len(p.Buffer)")
			w.lnPrint("switch p.Buffer[position] {")
			element := list.Front()
			for ; element.Next() != nil; element = element.Next() {
				sequence := element.Value.(List).Front()
				class := sequence.Value.(List).Front().Value.(Node).(Token).GetClass()
				sequence = sequence.Next()
				w.lnPrint("case")
				comma := false
				for d := 0; d < 256; d++ {
					if class.has(uint8(d)) {
						if comma {
							print(",")
						}
						s := ""
						switch uint8(d) {
						case '\a':
							s = `\a` /* bel */
						case '\b':
							s = `\b` /* bs */
						case '\f':
							s = `\f` /* ff */
						case '\n':
							s = `\n` /* nl */
						case '\r':
							s = `\r` /* cr */
						case '\t':
							s = `\t` /* ht */
						case '\v':
							s = `\v` /* vt */
						case '\\':
							s = `\\` /* \ */
						case '\'':
							s = `\'` /* ' */
						default:
							s = fmt.Sprintf("%c", d)
						}
						print(" '%s'", s)
						comma = true
					}
				}
				print(":")
				w.indent++
				updateFlags(compile(sequence.Value.(Node), done))
				w.indent--
			}
			w.lnPrint("default:")
			w.indent++
			updateFlags(compile(element.Value.(List).Front().Next().Value.(Node), done))
			w.indent--
			w.lnPrint("}")
			w.end()
			if ok.used {
				ok.label()
			}
		case TypeSequence:
			for element := node.(List).Front(); element != nil; element = element.Next() {
				updateFlags(compile(element.Value.(Node), ko))
			}
			if node.(List).Len() > 1 {
				if chgok.pos { chgko.pos = true }
				if chgok.thPos { chgko.thPos = true}
			}
		case TypePeekFor:
			sub := node.(List).Front().Value.(Node)
			if canCompilePeek(sub, false, ko) {
				return
			}
			l := w.newLabel()
			l.saveBlock()
			cko, cok := compile(sub, ko)
			l.lrestore(nil, cok.pos, cok.thPos)
			chgko = cko
		case TypePeekNot:
			sub := node.(List).Front().Value.(Node)
			if canCompilePeek(sub, true, ko) {
				return
			}
			ok := w.newLabel()
			ok.saveBlock()
			cko, cok := compile(sub, ok)
			ko.jump()
			if ok.used {
				ok.restore(cko.pos, cko.thPos)
			}
			chgko = cok
		case TypeQuery:
			qko := w.newLabel()
			qok := w.newLabel()
			qko.saveBlock()
			cko, cok := compile(node.(List).Front().Value.(Node), qko)
			if qko.unsafe() {
				qok.jump()
			}
			if qko.used {
				qko.restore(cko.pos, cko.thPos)
			}
			if qko.unsafe() {
				qok.label()
			}
			chgok = cok
		case TypeStar:
			again := w.newLabel()
			out := w.newLabel()
			again.label()
			out.saveBlock()
			cko, cok := compile(node.(List).Front().Value.(Node), out)
			again.jump()
			out.restore(cko.pos, cko.thPos)
			chgok = cok
		case TypePlus:
			again := w.newLabel()
			out := w.newLabel()
			updateFlags(compile(node.(List).Front().Value.(Node), ko))
			again.label()
			out.saveBlock()
			cko, _ := compile(node.(List).Front().Value.(Node), out)
			again.jump()
			if out.used {
				out.restore(cko.pos, cko.thPos)
			}
		case TypeNil:
		default:
			fmt.Fprintf(os.Stderr, "illegal node type: %v\n", node.GetType())
		}
		return
	}

	// dry compilation
	// figure out which items need to restore position resp. thunkPosition,
	// storing into w.saveFlags
	w.setDry(true)
	for element := t.Front(); element != nil; element = element.Next() {
		node := element.Value.(Node)
		if node.GetType() != TypeRule {
			continue
		}
		rule := node.(*rule)
		expression := rule.GetExpression()
		if expression == nilNode {
			continue
		}
		ko := w.newLabel()
		ko.sid = 0
		if count, ok := t.rulesCount[rule.String()]; !ok {
		} else if t.inline && count == 1 && ko.id != 0 {
			continue
		}
		ko.save()
		cko, _ := compileExpression(rule, ko)
		if ko.used {
			ko.restore(cko.pos, cko.thPos)
		}
	}
	w.setDry(false)

	/* now for the real compile pass */
	print("\n\tp.rules = [...]func() bool{")
	for element := t.Front(); element != nil; element = element.Next() {
		node := element.Value.(Node)
		if node.GetType() != TypeRule {
			continue
		}
		rule := node.(*rule)
		expression := rule.GetExpression()
		if expression == nilNode {
			fmt.Fprintf(os.Stderr, "rule '%v' used but not defined\n", rule)
			w.lnPrint("nil,")
			continue
		}
		ko := w.newLabel()
		ko.sid = 0
		w.lnPrint("/* %v ", rule.GetId())
		printRule(rule)
		print(" */")
		if count, ok := t.rulesCount[rule.String()]; !ok {
			fmt.Fprintf(os.Stderr, "rule '%v' defined but not used\n", rule)
		} else if t.inline && count == 1 && ko.id != 0 {
			w.lnPrint("nil,")
			continue
		}
		w.lnPrint("func() bool {")
		w.indent++
		ko.save()
		cko, _ := compileExpression(rule, ko)
		w.lnPrint("return true")
		if ko.used {
			ko.restore(cko.pos, cko.thPos)
			w.lnPrint("return false")
		}
		w.indent--
		w.lnPrint("},")
	}
	print("\n\t}")
	print("\n}\n")

	for _, s := range t.trailers {
		print("%s", s)
	}
}

type chgFlags struct {
	pos, thPos bool
}

type writer struct {
	io.Writer
	indent    int
	hasCommit bool
	nLabels   int
	dryRun bool
	savedIndent int
	saveFlags []saveFlags
}

type saveFlags struct {
	pos, thPos bool
}

func newWriter(out io.Writer) *writer {
	return &writer{Writer: out, indent: 2}
}

func (w *writer) begin() {
	w.lnPrint("{")
	w.indent++
}

func (w *writer) end() {
	w.indent--
	w.lnPrint("}")
}

func (w *writer) setDry(on bool) {
	w.dryRun = on
	if on {
		w.savedIndent = w.indent
	} else {
		w.indent = w.savedIndent
		w.nLabels = 0
	}
}

type label struct {
	id, sid int
	*writer
	used bool
	savedBlockOpen bool
}

func (w *writer) newLabel() *label {
	i := w.nLabels
	w.nLabels++
	if w.dryRun {
		w.saveFlags = append(w.saveFlags, saveFlags{})
	}
	return &label{id: i, sid: i, writer: w}
}

func (w *label) label() {
	w.indent--
	w.lnPrint("l%d:", w.id)
	w.indent++
}

func (w *label) jump() {
	w.lnPrint("goto l%d", w.id)
	w.used = true
}

func (w *label) saveBlock() {
	save := w.saveFlags[w.id]
	if save.pos || save.thPos {
		w.begin()
		w.save()
		w.savedBlockOpen = true
	}
}
func (w *label) save() {
	save := w.saveFlags[w.id]
	switch {
	case save.pos && save.thPos:
		w.lnPrint("position%d, thunkPosition%d := position, thunkPosition", w.sid, w.sid)
	case !save.pos && save.thPos:
		w.lnPrint("thunkPosition%d := thunkPosition", w.sid)
	case save.pos:
		w.lnPrint("position%d := position", w.sid)
	}
}

func (w *label) unsafe() bool {
	save := w.saveFlags[w.id]
	return save.pos || save.thPos
}

func (w *label) restore(savePos, saveThPos bool) {
	w.lrestore(w, savePos, saveThPos)
}
func (w *label) lrestore(label *label, savePos, saveThPos bool) {
	if label != nil {
		if label.used {
			label.label()
		}
	}
	switch {
	case savePos && saveThPos:
		w.lnPrint("position, thunkPosition = position%d, thunkPosition%d", w.sid, w.sid)
	case !savePos && saveThPos:
		w.lnPrint("thunkPosition = thunkPosition%d", w.sid)
	case savePos:
		w.lnPrint("position = position%d", w.sid)
	}
	if w.dryRun {
		save := &w.saveFlags[w.id]
		if !save.pos {
			save.pos = savePos
		}
		if !save.thPos {
			save.thPos = saveThPos
		}
	}
	if w.savedBlockOpen {
		w.end()
		w.savedBlockOpen = false
	}
}

func (w *label) cJump(jumpIfTrue bool, format string, a ...interface{}) {
	w.used = true
	if w.dryRun {
		return
	}
	if jumpIfTrue {
		format = "if " + format
	} else {
		format = "if !" + format
	}
	w.lnPrint(format, a...)
	fmt.Fprint(w, " {")
	w.lnPrint("\tgoto l%d", w.id)
	w.lnPrint("}")
}

func (w *writer) lnPrint(format string, a ...interface{}) {
	if w.dryRun {
		return
	}
	s := "\n"
	for i := 0; i < w.indent; i++ {
		s += "\t"
	}
	fmt.Fprintf(w, s+format, a...)
}
