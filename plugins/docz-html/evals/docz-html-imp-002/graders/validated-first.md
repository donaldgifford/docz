---
type: regex
target: trace
pattern: '"command":"(?:[^"\\]|\\.)*docz validate(?:[^"\\]|\\.)*docz export html|"command":"(?:[^"\\]|\\.)*docz validate[\s\S]*?"command":"(?:[^"\\]|\\.)*docz export html'
---

docz validate ran before docz export html, in one command or two.
