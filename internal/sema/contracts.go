package sema

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bongdo/ink-lang/internal/ast"
)

func (p *Program) resolveContracts(fc *fileCtx, d *ast.TypeDecl) {
	if len(d.Contracts) == 0 {
		return
	}
	if len(d.TypeParams) > 0 {
		p.errorf(fc.file.Path, d.Pos(), "contract: blocks are not supported on generic types")
		for _, c := range d.Contracts {
			p.invalidateContract(c)
		}
		return
	}
	fields := make(map[string]*ast.Field, len(d.Fields))
	for _, f := range d.Fields {
		fields[f.Name] = f
	}
	methods := make(map[string]*ast.FuncDecl, len(d.Methods))
	for _, m := range d.Methods {
		methods[m.Name] = m
	}
	seen := make(map[string]*ast.Contract, len(d.Contracts))
	for _, c := range d.Contracts {
		bad := false
		if !checkDeclName(p, fc.file, c.Name, c.Pos()) {
			bad = true
		}
		if prev, ok := seen[c.Name]; ok {
			p.errorf(fc.file.Path, c.Pos(), "contract: block %q is already declared at line %d", c.Name, prev.Pos().Line)
			p.invalidateContract(c)
			continue
		}
		seen[c.Name] = c
		if c.Name != "wire" {
			r, _ := utf8.DecodeRuneInString(c.Name)
			if !unicode.IsLower(r) {
				p.errorf(fc.file.Path, c.Pos(), "contract: named view %q must start with a lowercase letter", c.Name)
				bad = true
			}
			if _, ok := fields[c.Name]; ok {
				p.errorf(fc.file.Path, c.Pos(), "contract: view %q collides with a field", c.Name)
				bad = true
			}
			if _, ok := methods[c.Name]; ok {
				p.errorf(fc.file.Path, c.Pos(), "contract: view %q collides with a method", c.Name)
				bad = true
			}
		}
		if len(c.Fields) == 0 {
			p.errorf(fc.file.Path, c.Pos(), "contract: block %q is empty", c.Name)
			bad = true
		}
		usedFields := make(map[string]bool, len(c.Fields))
		usedNames := make(map[string]bool, len(c.Fields))
		for _, cf := range c.Fields {
			if cf.Field == "_" {
				p.errorf(fc.file.Path, cf.Pos(), "contract: blank field is not allowed")
				bad = true
				continue
			}
			field, ok := fields[cf.Field]
			if !ok {
				p.errorf(fc.file.Path, cf.Pos(), "contract: field %q is not in %s", cf.Field, d.Name)
				bad = true
				continue
			}
			if !exported(field.Name) {
				p.errorf(fc.file.Path, cf.Pos(), "contract: field %q is not exported; JSON cannot encode it", cf.Field)
				bad = true
				continue
			}
			if usedFields[cf.Field] {
				p.errorf(fc.file.Path, cf.Pos(), "contract: field %q is listed twice", cf.Field)
				bad = true
				continue
			}
			usedFields[cf.Field] = true
			if !validTagName(cf.External) {
				p.errorf(fc.file.Path, cf.Pos(), "contract: %q cannot be used as a JSON name", cf.External)
				bad = true
				continue
			}
			if usedNames[cf.External] {
				p.errorf(fc.file.Path, cf.Pos(), "contract: name %q is used twice", cf.External)
				bad = true
				continue
			}
			usedNames[cf.External] = true
		}
		if bad {
			p.invalidateContract(c)
		}
	}
}

func (p *Program) invalidateContract(c *ast.Contract) {
	if p.invalidContracts == nil {
		p.invalidContracts = make(map[*ast.Contract]bool)
	}
	p.invalidContracts[c] = true
}

func validTagName(s string) bool {
	if s == "" || s == "-" {
		return false
	}
	for _, r := range s {
		if strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", r) {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func contractView(d *ast.TypeDecl, name string) *ast.Contract {
	for _, c := range d.Contracts {
		if c.Name == name && c.Name != "wire" {
			return c
		}
	}
	return nil
}

func (p *Program) markContractUse(d *ast.TypeDecl, name string) {
	if p.contractUses == nil {
		p.contractUses = make(map[*ast.TypeDecl]map[string]bool)
	}
	if p.contractUses[d] == nil {
		p.contractUses[d] = make(map[string]bool)
	}
	p.contractUses[d][name] = true
}

func (p *Program) checkContractUsage() {
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				td, ok := d.(*ast.TypeDecl)
				if !ok {
					continue
				}
				for _, c := range td.Contracts {
					if c.Name == "wire" || p.invalidContracts[c] {
						continue
					}
					if !p.contractUses[td][c.Name] {
						p.errorf(f.Path, c.Pos(), "contract: named view %q is unused", c.Name)
					}
				}
			}
		}
	}
}
