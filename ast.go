// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package dst declares the types used to represent syntax trees for Go
// packages.
//
package dst

import (
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ----------------------------------------------------------------------------
// Interfaces
//
// There are 3 main classes of nodes: Expressions and type nodes,
// statement nodes, and declaration nodes. The node names usually
// match the corresponding Go spec production names to which they
// correspond. The node fields correspond to the individual parts
// of the respective productions.
//
// All nodes contain position information marking the beginning of
// the corresponding source text segment; it is accessible via the
// Pos accessor method. Nodes may contain additional position info
// for language constructs where comments may be found between parts
// of the construct (typically any larger, parenthesized subpart).
// That position information is needed to properly position comments
// when printing the construct.

// All node types implement the Node interface.
type Node interface {
	Decorations() []Decoration
}

type Decoration struct {
	Position string
	End      bool
	Text     string
}

// All expression nodes implement the Expr interface.
type Expr interface {
	Node
	exprNode()
}

// All statement nodes implement the Stmt interface.
type Stmt interface {
	Node
	stmtNode()
}

// All declaration nodes implement the Decl interface.
type Decl interface {
	Node
	declNode()
}

// ----------------------------------------------------------------------------
// Comments

// A Comment node represents a single //-style or /*-style comment.
type Comment struct {
	Text string // comment text (excluding '\n' for //-style comments)
	Decs []Decoration
}

// A CommentGroup represents a sequence of comments
// with no other tokens and no empty lines between.
//
type CommentGroup struct {
	List []*Comment // len(List) > 0
	Decs []Decoration
}

func isWhitespace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' }

func stripTrailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(s[i-1]) {
		i--
	}
	return s[0:i]
}

// Text returns the text of the comment.
// Comment markers (//, /*, and */), the first space of a line comment, and
// leading and trailing empty lines are removed. Multiple empty lines are
// reduced to one, and trailing space on lines is trimmed. Unless the result
// is empty, it is newline-terminated.
//
func (g *CommentGroup) Text() string {
	if g == nil {
		return ""
	}
	comments := make([]string, len(g.List))
	for i, c := range g.List {
		comments[i] = c.Text
	}

	lines := make([]string, 0, 10) // most comments are less than 10 lines
	for _, c := range comments {
		// Remove comment markers.
		// The parser has given us exactly the comment text.
		switch c[1] {
		case '/':
			//-style comment (no newline at the end)
			c = c[2:]
			// strip first space - required for Example tests
			if len(c) > 0 && c[0] == ' ' {
				c = c[1:]
			}
		case '*':
			/*-style comment */
			c = c[2 : len(c)-2]
		}

		// Split on newlines.
		cl := strings.Split(c, "\n")

		// Walk lines, stripping trailing white space and adding to list.
		for _, l := range cl {
			lines = append(lines, stripTrailingWhitespace(l))
		}
	}

	// Remove leading blank lines; convert runs of
	// interior blank lines to a single blank line.
	n := 0
	for _, line := range lines {
		if line != "" || n > 0 && lines[n-1] != "" {
			lines[n] = line
			n++
		}
	}
	lines = lines[0:n]

	// Add final "" entry to get trailing newline from Join.
	if n > 0 && lines[n-1] != "" {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// ----------------------------------------------------------------------------
// Expressions and types

// A Field represents a Field declaration list in a struct type,
// a method list in an interface type, or a parameter/result declaration
// in a signature.
// Field.Names is nil for unnamed parameters (parameter lists which only contain types)
// and embedded struct fields. In the latter case, the field name is the type name.
//
type Field struct {
	Doc     *CommentGroup // associated documentation; or nil
	Names   []*Ident      // field/method/parameter names; or nil
	Type    Expr          // field/method/parameter type
	Tag     *BasicLit     // field tag; or nil
	Comment *CommentGroup // line comments; or nil
	Decs    []Decoration
}

// A FieldList represents a list of Fields, enclosed by parentheses or braces.
type FieldList struct {
	List []*Field // field list; or nil
	Decs []Decoration
}

// NumFields returns the number of parameters or struct fields represented by a FieldList.
func (f *FieldList) NumFields() int {
	n := 0
	if f != nil {
		for _, g := range f.List {
			m := len(g.Names)
			if m == 0 {
				m = 1
			}
			n += m
		}
	}
	return n
}

// An expression is represented by a tree consisting of one
// or more of the following concrete expression nodes.
//
type (
	// A BadExpr node is a placeholder for expressions containing
	// syntax errors for which no correct expression nodes can be
	// created.
	//
	BadExpr struct {
		Length int // position range of bad expression
		Decs   []Decoration
	}

	// An Ident node represents an identifier.
	Ident struct {
		Name string  // identifier name
		Obj  *Object // denoted object; or nil
		Decs []Decoration
	}

	// An Ellipsis node stands for the "..." type in a
	// parameter list or the "..." length in an array type.
	//
	Ellipsis struct {
		Elt  Expr // ellipsis element type (parameter lists only); or nil
		Decs []Decoration
	}

	// A BasicLit node represents a literal of basic type.
	BasicLit struct {
		Kind  token.Token // token.INT, token.FLOAT, token.IMAG, token.CHAR, or token.STRING
		Value string      // literal string; e.g. 42, 0x7f, 3.14, 1e-9, 2.4i, 'a', '\x7f', "foo" or `\m\n\o`
		Decs  []Decoration
	}

	// A FuncLit node represents a function literal.
	FuncLit struct {
		Type *FuncType  // function type
		Body *BlockStmt // function body
		Decs []Decoration
	}

	// A CompositeLit node represents a composite literal.
	CompositeLit struct {
		Type       Expr   // literal type; or nil
		Elts       []Expr // list of composite elements; or nil
		Incomplete bool   // true if (source) expressions are missing in the Elts list
		Decs       []Decoration
	}

	// A ParenExpr node represents a parenthesized expression.
	ParenExpr struct {
		X    Expr // parenthesized expression
		Decs []Decoration
	}

	// A SelectorExpr node represents an expression followed by a selector.
	SelectorExpr struct {
		X    Expr   // expression
		Sel  *Ident // field selector
		Decs []Decoration
	}

	// An IndexExpr node represents an expression followed by an index.
	IndexExpr struct {
		X     Expr // expression
		Index Expr // index expression
		Decs  []Decoration
	}

	// An SliceExpr node represents an expression followed by slice indices.
	SliceExpr struct {
		X      Expr // expression
		Low    Expr // begin of slice range; or nil
		High   Expr // end of slice range; or nil
		Max    Expr // maximum capacity of slice; or nil
		Slice3 bool // true if 3-index slice (2 colons present)
		Decs   []Decoration
	}

	// A TypeAssertExpr node represents an expression followed by a
	// type assertion.
	//
	TypeAssertExpr struct {
		X    Expr // expression
		Type Expr // asserted type; nil means type switch X.(type)
		Decs []Decoration
	}

	// A CallExpr node represents an expression followed by an argument list.
	CallExpr struct {
		Fun      Expr   // function expression
		Args     []Expr // function arguments; or nil
		Ellipsis bool
		Decs     []Decoration
	}

	// A StarExpr node represents an expression of the form "*" Expression.
	// Semantically it could be a unary "*" expression, or a pointer type.
	//
	StarExpr struct {
		X    Expr // operand
		Decs []Decoration
	}

	// A UnaryExpr node represents a unary expression.
	// Unary "*" expressions are represented via StarExpr nodes.
	//
	UnaryExpr struct {
		Op   token.Token // operator
		X    Expr        // operand
		Decs []Decoration
	}

	// A BinaryExpr node represents a binary expression.
	BinaryExpr struct {
		X    Expr        // left operand
		Op   token.Token // operator
		Y    Expr        // right operand
		Decs []Decoration
	}

	// A KeyValueExpr node represents (key : value) pairs
	// in composite literals.
	//
	KeyValueExpr struct {
		Key   Expr
		Value Expr
		Decs  []Decoration
	}
)

// The direction of a channel type is indicated by a bit
// mask including one or both of the following constants.
//
type ChanDir int

const (
	SEND ChanDir = 1 << iota
	RECV
)

// A type is represented by a tree consisting of one
// or more of the following type-specific expression
// nodes.
//
type (
	// An ArrayType node represents an array or slice type.
	ArrayType struct {
		Len  Expr // Ellipsis node for [...]T array types, nil for slice types
		Elt  Expr // element type
		Decs []Decoration
	}

	// A StructType node represents a struct type.
	StructType struct {
		Fields     *FieldList // list of field declarations
		Incomplete bool       // true if (source) fields are missing in the Fields list
		Decs       []Decoration
	}

	// Pointer types are represented via StarExpr nodes.

	// A FuncType node represents a function type.
	FuncType struct {
		Func    bool
		Params  *FieldList // (incoming) parameters; non-nil
		Results *FieldList // (outgoing) results; or nil
		Decs    []Decoration
	}

	// An InterfaceType node represents an interface type.
	InterfaceType struct {
		Methods    *FieldList // list of methods
		Incomplete bool       // true if (source) methods are missing in the Methods list
		Decs       []Decoration
	}

	// A MapType node represents a map type.
	MapType struct {
		Key   Expr
		Value Expr
		Decs  []Decoration
	}

	// A ChanType node represents a channel type.
	ChanType struct {
		Dir   ChanDir // channel direction
		Value Expr    // value type
		Decs  []Decoration
	}
)

// exprNode() ensures that only expression/type nodes can be
// assigned to an Expr.
//
func (*BadExpr) exprNode()        {}
func (*Ident) exprNode()          {}
func (*Ellipsis) exprNode()       {}
func (*BasicLit) exprNode()       {}
func (*FuncLit) exprNode()        {}
func (*CompositeLit) exprNode()   {}
func (*ParenExpr) exprNode()      {}
func (*SelectorExpr) exprNode()   {}
func (*IndexExpr) exprNode()      {}
func (*SliceExpr) exprNode()      {}
func (*TypeAssertExpr) exprNode() {}
func (*CallExpr) exprNode()       {}
func (*StarExpr) exprNode()       {}
func (*UnaryExpr) exprNode()      {}
func (*BinaryExpr) exprNode()     {}
func (*KeyValueExpr) exprNode()   {}

func (*ArrayType) exprNode()     {}
func (*StructType) exprNode()    {}
func (*FuncType) exprNode()      {}
func (*InterfaceType) exprNode() {}
func (*MapType) exprNode()       {}
func (*ChanType) exprNode()      {}

// ----------------------------------------------------------------------------
// Convenience functions for Idents

// NewIdent creates a new Ident without position.
// Useful for ASTs generated by code other than the Go parser.
//
func NewIdent(name string) *Ident { return &Ident{name, nil, nil} }

// IsExported reports whether name is an exported Go symbol
// (that is, whether it begins with an upper-case letter).
//
func IsExported(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(ch)
}

// IsExported reports whether id is an exported Go symbol
// (that is, whether it begins with an uppercase letter).
//
func (id *Ident) IsExported() bool { return IsExported(id.Name) }

func (id *Ident) String() string {
	if id != nil {
		return id.Name
	}
	return "<nil>"
}

// ----------------------------------------------------------------------------
// Statements

// A statement is represented by a tree consisting of one
// or more of the following concrete statement nodes.
//
type (
	// CommentStmt is for comments in a statement list that are
	// separated from other statements by a blank line.
	CommentStmt struct {
		Comment *CommentGroup
		Decs    []Decoration
	}

	// A BadStmt node is a placeholder for statements containing
	// syntax errors for which no correct statement nodes can be
	// created.
	//
	BadStmt struct {
		Length int // position range of bad statement
		Decs   []Decoration
	}

	// A DeclStmt node represents a declaration in a statement list.
	DeclStmt struct {
		Decl Decl // *GenDecl with CONST, TYPE, or VAR token
		Decs []Decoration
	}

	// An EmptyStmt node represents an empty statement.
	// The "position" of the empty statement is the position
	// of the immediately following (explicit or implicit) semicolon.
	//
	EmptyStmt struct {
		Implicit bool // if set, ";" was omitted in the source
		Decs     []Decoration
	}

	// A LabeledStmt node represents a labeled statement.
	LabeledStmt struct {
		Label *Ident
		Stmt  Stmt
		Decs  []Decoration
	}

	// An ExprStmt node represents a (stand-alone) expression
	// in a statement list.
	//
	ExprStmt struct {
		X    Expr // expression
		Decs []Decoration
	}

	// A SendStmt node represents a send statement.
	SendStmt struct {
		Chan  Expr
		Value Expr
		Decs  []Decoration
	}

	// An IncDecStmt node represents an increment or decrement statement.
	IncDecStmt struct {
		X    Expr
		Tok  token.Token // INC or DEC
		Decs []Decoration
	}

	// An AssignStmt node represents an assignment or
	// a short variable declaration.
	//
	AssignStmt struct {
		Lhs  []Expr
		Tok  token.Token // assignment token, DEFINE
		Rhs  []Expr
		Decs []Decoration
	}

	// A GoStmt node represents a go statement.
	GoStmt struct {
		Call *CallExpr
		Decs []Decoration
	}

	// A DeferStmt node represents a defer statement.
	DeferStmt struct {
		Call *CallExpr
		Decs []Decoration
	}

	// A ReturnStmt node represents a return statement.
	ReturnStmt struct {
		Results []Expr // result expressions; or nil
		Decs    []Decoration
	}

	// A BranchStmt node represents a break, continue, goto,
	// or fallthrough statement.
	//
	BranchStmt struct {
		Tok   token.Token // keyword token (BREAK, CONTINUE, GOTO, FALLTHROUGH)
		Label *Ident      // label name; or nil
		Decs  []Decoration
	}

	// A BlockStmt node represents a braced statement list.
	BlockStmt struct {
		List []Stmt
		Decs []Decoration
	}

	// An IfStmt node represents an if statement.
	IfStmt struct {
		Init Stmt // initialization statement; or nil
		Cond Expr // condition
		Body *BlockStmt
		Else Stmt // else branch; or nil
		Decs []Decoration
	}

	// A CaseClause represents a case of an expression or type switch statement.
	CaseClause struct {
		List []Expr // list of expressions or types; nil means default case
		Body []Stmt // statement list; or nil
		Decs []Decoration
	}

	// A SwitchStmt node represents an expression switch statement.
	SwitchStmt struct {
		Init Stmt       // initialization statement; or nil
		Tag  Expr       // tag expression; or nil
		Body *BlockStmt // CaseClauses only
		Decs []Decoration
	}

	// An TypeSwitchStmt node represents a type switch statement.
	TypeSwitchStmt struct {
		Init   Stmt       // initialization statement; or nil
		Assign Stmt       // x := y.(type) or y.(type)
		Body   *BlockStmt // CaseClauses only
		Decs   []Decoration
	}

	// A CommClause node represents a case of a select statement.
	CommClause struct {
		Comm Stmt   // send or receive statement; nil means default case
		Body []Stmt // statement list; or nil
		Decs []Decoration
	}

	// An SelectStmt node represents a select statement.
	SelectStmt struct {
		Body *BlockStmt // CommClauses only
		Decs []Decoration
	}

	// A ForStmt represents a for statement.
	ForStmt struct {
		Init Stmt // initialization statement; or nil
		Cond Expr // condition; or nil
		Post Stmt // post iteration statement; or nil
		Body *BlockStmt
		Decs []Decoration
	}

	// A RangeStmt represents a for statement with a range clause.
	RangeStmt struct {
		Key, Value Expr        // Key, Value may be nil
		Tok        token.Token // ILLEGAL if Key == nil, ASSIGN, DEFINE
		X          Expr        // value to range over
		Body       *BlockStmt
		Decs       []Decoration
	}
)

// stmtNode() ensures that only statement nodes can be
// assigned to a Stmt.
//
func (*BadStmt) stmtNode()        {}
func (*DeclStmt) stmtNode()       {}
func (*EmptyStmt) stmtNode()      {}
func (*LabeledStmt) stmtNode()    {}
func (*ExprStmt) stmtNode()       {}
func (*SendStmt) stmtNode()       {}
func (*IncDecStmt) stmtNode()     {}
func (*AssignStmt) stmtNode()     {}
func (*GoStmt) stmtNode()         {}
func (*DeferStmt) stmtNode()      {}
func (*ReturnStmt) stmtNode()     {}
func (*BranchStmt) stmtNode()     {}
func (*BlockStmt) stmtNode()      {}
func (*IfStmt) stmtNode()         {}
func (*CaseClause) stmtNode()     {}
func (*SwitchStmt) stmtNode()     {}
func (*TypeSwitchStmt) stmtNode() {}
func (*CommClause) stmtNode()     {}
func (*SelectStmt) stmtNode()     {}
func (*ForStmt) stmtNode()        {}
func (*RangeStmt) stmtNode()      {}
func (*CommentStmt) stmtNode()    {}

// ----------------------------------------------------------------------------
// Declarations

// A Spec node represents a single (non-parenthesized) import,
// constant, type, or variable declaration.
//
type (
	// The Spec type stands for any of *ImportSpec, *ValueSpec, and *TypeSpec.
	Spec interface {
		Node
		specNode()
	}

	// An ImportSpec node represents a single package import.
	ImportSpec struct {
		Doc     *CommentGroup // associated documentation; or nil
		Name    *Ident        // local package name (including "."); or nil
		Path    *BasicLit     // import path
		Comment *CommentGroup // line comments; or nil
		Decs    []Decoration
	}

	// A ValueSpec node represents a constant or variable declaration
	// (ConstSpec or VarSpec production).
	//
	ValueSpec struct {
		Doc     *CommentGroup // associated documentation; or nil
		Names   []*Ident      // value names (len(Names) > 0)
		Type    Expr          // value type; or nil
		Values  []Expr        // initial values; or nil
		Comment *CommentGroup // line comments; or nil
		Decs    []Decoration
	}

	// A TypeSpec node represents a type declaration (TypeSpec production).
	TypeSpec struct {
		Doc     *CommentGroup // associated documentation; or nil
		Name    *Ident        // type name
		Assign  bool          // position of '=', if any
		Type    Expr          // *Ident, *ParenExpr, *SelectorExpr, *StarExpr, or any of the *XxxTypes
		Comment *CommentGroup // line comments; or nil
		Decs    []Decoration
	}
)

// Pos and End implementations for spec nodes.

// specNode() ensures that only spec nodes can be
// assigned to a Spec.
//
func (*ImportSpec) specNode() {}
func (*ValueSpec) specNode()  {}
func (*TypeSpec) specNode()   {}

// A declaration is represented by one of the following declaration nodes.
//
type (
	// A BadDecl node is a placeholder for declarations containing
	// syntax errors for which no correct declaration nodes can be
	// created.
	//
	BadDecl struct {
		Length int // position range of bad declaration
		Decs   []Decoration
	}

	// A GenDecl node (generic declaration node) represents an import,
	// constant, type or variable declaration. A valid Lparen position
	// (Lparen.IsValid()) indicates a parenthesized declaration.
	//
	// Relationship between Tok value and Specs element type:
	//
	//	token.IMPORT  *ImportSpec
	//	token.CONST   *ValueSpec
	//	token.TYPE    *TypeSpec
	//	token.VAR     *ValueSpec
	//
	GenDecl struct {
		Doc    *CommentGroup // associated documentation; or nil
		Tok    token.Token   // IMPORT, CONST, TYPE, VAR
		Lparen bool
		Specs  []Spec
		Rparen bool
		Decs   []Decoration
	}

	// A FuncDecl node represents a function declaration.
	FuncDecl struct {
		Doc  *CommentGroup // associated documentation; or nil
		Recv *FieldList    // receiver (methods); or nil (functions)
		Name *Ident        // function/method name
		Type *FuncType     // function signature: parameters, results, and position of "func" keyword
		Body *BlockStmt    // function body; or nil for external (non-Go) function
		Decs []Decoration
	}
)

// declNode() ensures that only declaration nodes can be
// assigned to a Decl.
//
func (*BadDecl) declNode()  {}
func (*GenDecl) declNode()  {}
func (*FuncDecl) declNode() {}

// ----------------------------------------------------------------------------
// Files and packages

// A File node represents a Go source file.
//
// The Comments list contains all comments in the source file in order of
// appearance, including the comments that are pointed to from other nodes
// via Doc and Comment fields.
//
// For correct printing of source code containing comments (using packages
// go/format and go/printer), special care must be taken to update comments
// when a File's syntax tree is modified: For printing, comments are interspersed
// between tokens based on their position. If syntax tree nodes are
// removed or moved, relevant comments in their vicinity must also be removed
// (from the File.Comments list) or moved accordingly (by updating their
// positions). A CommentMap may be used to facilitate some of these operations.
//
// Whether and how a comment is associated with a node depends on the
// interpretation of the syntax tree by the manipulating program: Except for Doc
// and Comment comments directly associated with nodes, the remaining comments
// are "free-floating" (see also issues #18593, #20744).
//
type File struct {
	Doc        *CommentGroup   // associated documentation; or nil
	Name       *Ident          // package name
	Decls      []Decl          // top-level declarations; or nil
	Scope      *Scope          // package scope (this file only)
	Imports    []*ImportSpec   // imports in this file
	Unresolved []*Ident        // unresolved identifiers in this file
	Comments   []*CommentGroup // list of all comments in the source file
	Decs       []Decoration
}

// A Package node represents a set of source files
// collectively building a Go package.
//
type Package struct {
	Name    string             // package name
	Scope   *Scope             // package scope across all files
	Imports map[string]*Object // map of package id -> package object
	Files   map[string]*File   // Go source files by filename
}

func (v *Package) Decorations() []Decoration {
	return nil
}
