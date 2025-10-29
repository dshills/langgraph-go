# Medium Test Fixture

This directory contains 100 sample Go files organized into 5 packages for testing the multi-LLM code review system.

## Structure

- **pkg1/** (20 files): Models and Services
  - User, Product, Order models
  - Account, Payment, Inventory, Notification services
  - Issues: Missing validation, incomplete error handling

- **pkg2/** (20 files): Handlers and Middleware
  - HTTP handlers numbered 1-20
  - Issues: Missing error handling, incomplete validation

- **pkg3/** (20 files): Utilities and Helpers
  - String parsing, calculations, time formatting
  - Issues: No validation, division by zero, hardcoded formats

- **pkg4/** (20 files): Database and Repository
  - Repository implementations numbered 1-20
  - Issues: SQL injection vulnerabilities, missing error handling

- **pkg5/** (20 files): Configuration and Validation
  - Configuration loaders numbered 1-20
  - Issues: Hardcoded values, incomplete validation, missing error handling

## Intentional Code Issues

Each file contains 15-25 lines of valid Go code with intentional issues including:
- Missing error handling
- Potential nil pointer dereferences
- Inefficient algorithms
- Hardcoded values (API keys, tax rates, formats)
- Missing input validation
- Unclosed resources
- SQL injection vulnerabilities
- Thread safety issues
- Division by zero potential

## Usage

These files are designed to test the multi-LLM code review workflow by providing a realistic medium-sized codebase with various code quality issues for LLMs to identify and analyze.
