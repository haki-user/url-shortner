# Go Learning Handbook

This handbook explains Go through the TinyURL project. It is organized so you can either learn sequentially or quickly look up a concept while coding.

## Recommended Reading Order

1. [Go Mental Model](mental-model.md)  
   Start here. Explains how Go differs from JavaScript and how packages, structs, methods, values, pointers, and interfaces fit together.

2. [Pointers, Methods, and Interfaces](pointers-methods-interfaces.md)  
   Detailed answers to the questions that came up while implementing the domain and ports layers.

3. [TinyURL Architecture Context](tinyurl-architecture-context.md)  
   Connects Go language features to this repository's domain, ports, application, and adapter layers.

4. [Go Cheat Sheet](cheat-sheet.md)  
   Quick syntax and decision reference to keep open while coding.

5. [Question Log](question-log.md)  
   A living record of doubts, concise answers, and concepts worth revisiting.

## How to Use This Handbook

When beginning a task:

1. Read its context and acceptance criteria.
2. Look up unfamiliar syntax in the cheat sheet.
3. Use the deeper documents when the reason behind a pattern is unclear.
4. Add recurring questions to the question log.
5. Explain the behavior in plain English before writing code.

The goal is not to memorize Go syntax. The goal is to build a reliable mental model that lets you predict what the code will do.

## Learning Loop

Use this loop for each small feature:

```text
Understand the behavior
-> predict the implementation
-> write a small test
-> implement
-> run test and vet
-> explain failures
-> record new insight
```

Useful commands from the project root:

```powershell
$env:GOCACHE="$PWD\.cache\go-build"
& "C:\Program Files\Go\bin\go.exe" test ./internal/link/...
& "C:\Program Files\Go\bin\go.exe" vet ./internal/link/...
& "C:\Program Files\Go\bin\gofmt.exe" -w <files>
```
