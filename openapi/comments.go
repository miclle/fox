package openapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

type commentDocs struct {
	funcs  map[string]string
	fields map[string]map[string]string
}

func newCommentDocs() *commentDocs {
	return &commentDocs{
		funcs:  make(map[string]string),
		fields: make(map[string]map[string]string),
	}
}

// Source enables Go comment extraction from the provided paths.
func Source(paths ...string) Option {
	return func(g *Generator) {
		docs := newCommentDocs()
		for _, path := range paths {
			if err := docs.load(path); err != nil {
				g.warnf("openapi source %q: %v", path, err)
			}
		}
		g.docs = docs
	}
}

func (d *commentDocs) load(path string) error {
	if strings.HasSuffix(path, "/...") {
		return d.loadTree(strings.TrimSuffix(path, "/..."))
	}
	return d.loadDir(path)
}

func (d *commentDocs) loadTree(root string) error {
	root = filepath.Clean(root)
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if path != root && (name == "vendor" || strings.HasPrefix(name, ".")) {
			return filepath.SkipDir
		}
		return d.loadDir(path)
	})
}

func (d *commentDocs) loadDir(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	if err != nil {
		return err
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			d.addFile(file)
		}
	}
	return nil
}

func (d *commentDocs) addFile(file *ast.File) {
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if decl.Doc != nil {
				d.funcs[decl.Name.Name] = commentText(decl.Doc)
			}
		case *ast.GenDecl:
			d.addTypeDecl(decl)
		}
	}
}

func (d *commentDocs) addTypeDecl(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		for _, field := range structType.Fields.List {
			group := field.Doc
			if group == nil {
				group = field.Comment
			}
			if group == nil {
				continue
			}
			text := commentText(group)
			for _, name := range field.Names {
				if d.fields[typeSpec.Name.Name] == nil {
					d.fields[typeSpec.Name.Name] = make(map[string]string)
				}
				d.fields[typeSpec.Name.Name][name.Name] = text
			}
		}
	}
}

func (d *commentDocs) funcDoc(runtimeName string) string {
	if runtimeName == "" {
		return ""
	}
	name := runtimeName
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return d.funcs[name]
}

func (d *commentDocs) fieldDoc(typeName, fieldName string) string {
	if d == nil {
		return ""
	}
	return d.fields[typeName][fieldName]
}

func commentText(group *ast.CommentGroup) string {
	return strings.TrimSpace(group.Text())
}

func firstParagraph(text string) string {
	if idx := strings.Index(text, "\n\n"); idx >= 0 {
		return text[:idx]
	}
	return text
}
