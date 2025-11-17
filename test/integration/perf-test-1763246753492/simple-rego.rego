package containerization.perf

violations contains result if {
  not regex.match("USER", input.content)
  result := {
    "rule": "require-user",
    "category": "security",
    "severity": "block",
    "message": "Must specify USER directive"
  }
}
