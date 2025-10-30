// Package main provides helper utilities for systematic code fixes
// This file contains common patterns used across multiple fix batches
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// Helper Note: This file provides documentation and patterns for manual fixes.
// Automated AST manipulation would require more sophisticated tooling.

// Common Fix Patterns Documentation

// Pattern 1: Division-by-Zero Check
// BEFORE:
//   func Calculate(x, y int) int {
//       return x / y
//   }
// AFTER:
//   func Calculate(x, y int) (int, error) {
//       if y == 0 {
//           return 0, fmt.Errorf("division by zero: cannot divide %d by zero", x)
//       }
//       return x / y, nil
//   }

// Pattern 2: Nil Pointer Check
// BEFORE:
//   func (m *Model) Process() {
//       m.ID = generateID()
//   }
// AFTER:
//   func (m *Model) Process() error {
//       if m == nil {
//           return fmt.Errorf("cannot process nil Model")
//       }
//       m.ID = generateID()
//       return nil
//   }

// Pattern 3: Error Handling
// BEFORE:
//   result := DoSomething()
// AFTER:
//   result, err := DoSomething()
//   if err != nil {
//       return fmt.Errorf("failed to do something: %w", err)
//   }

// Pattern 4: Resource Cleanup with Defer
// BEFORE:
//   file, _ := os.Open("file.txt")
//   data, _ := io.ReadAll(file)
//   file.Close()
// AFTER:
//   file, err := os.Open("file.txt")
//   if err != nil {
//       return err
//   }
//   defer file.Close()
//   data, err := io.ReadAll(file)
//   if err != nil {
//       return err
//   }

// Pattern 5: Context Usage
// BEFORE:
//   func LongRunningOp() error {
//       // operation
//   }
// AFTER:
//   func LongRunningOp(ctx context.Context) error {
//       select {
//       case <-ctx.Done():
//           return ctx.Err()
//       default:
//           // operation
//       }
//   }

// FindDivisionOperations finds all division operations in a Go file
// This can help identify locations needing zero-divisor checks
func FindDivisionOperations(filename string) ([]token.Position, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var positions []token.Position
	ast.Inspect(node, func(n ast.Node) bool {
		if binaryExpr, ok := n.(*ast.BinaryExpr); ok {
			if binaryExpr.Op == token.QUO || binaryExpr.Op == token.QUO_ASSIGN {
				positions = append(positions, fset.Position(binaryExpr.Pos()))
			}
		}
		return true
	})

	return positions, nil
}

// Example usage documentation
func exampleUsage() {
	// Find division operations in a file
	positions, err := FindDivisionOperations("graph/engine.go")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d division operations:\n", len(positions))
	for _, pos := range positions {
		fmt.Printf("  %s\n", pos)
	}
}
