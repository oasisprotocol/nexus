#!/usr/bin/python3

"""
Reads a Go source file on stdin and strips out
most functions, interfaces, and global variables,
essentially leaving only const declarations,
non-interface types, and their (un)marshaling functions.

Heuristic, hacky, only mostly correct, assumes a
gofumpt-ed input 
"""

import sys
import re

lines = sys.stdin.readlines()
out = []

in_func = False  # are we inside a function or other block we want to skip
for i, line in enumerate(lines):
  if in_func and i <= end:
    continue
  else:
    in_func = False

  if line.startswith("func"):
    # We'll remove functions in general, but we'll keep the (un)marshalers
    if "Marshal" in line or "Unmarshal" in line \
    or re.match(r"^func \([^)]+\) String\(\).*", line) \
    or "func (s SlashReason) checkedString()" in line:  # A dependency of a String(). This is a (possibly Cobalt-specific) hack.
      out.append(line)
      continue
    end = lines.index("}\n", i)
    in_func = True
    out.append("// removed func\n")
  elif re.match(r"^type.*interface \{$", line):
    end = lines.index("}\n", i)
    in_func = True
    out.append("// removed interface\n")
  elif re.match(r"^var.*=.*\{$", line):
    end = lines.index("}\n", i)
    in_func = True
    out.append("// removed var statement\n")
  elif line.startswith("var (") or re.match(r"^var.*=.*\($", line):
    end = lines.index(")\n", i)
    in_func = True
    out.append("// removed var block\n")
  elif re.match(r"^var.*=.*[^(]$", line):
    out.append("// removed var statement\n")
  else:
    out.append(line)

print("".join(out))
