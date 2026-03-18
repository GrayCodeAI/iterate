---
name: self-assess
description: Evaluate your own code quality and identify gaps
scope: [analysis, testing, quality]
---

# Self-Assess Skill

Regularly evaluate iterate's quality and identify improvement opportunities.

## Assessment Checklist

- [ ] **Code quality**: Run `go vet ./...`, fix any issues
- [ ] **Test coverage**: Is coverage > 60%? Add tests for gaps
- [ ] **Build**: Does `go build ./...` succeed?
- [ ] **Documentation**: Are commands documented in CLAUDE.md?
- [ ] **Consistency**: Do new commands follow existing patterns?
- [ ] **Performance**: Are there obvious inefficiencies?
- [ ] **Error handling**: Are errors graceful or do they crash?
- [ ] **Unused code**: Are there unused functions/variables?

## Gap Analysis

Look for:
1. **Missing features compared to yoyo-evolve**
   - Check README.md for feature list gaps
   - Evaluate commands that are partially implemented
2. **UX improvements**
   - Are error messages helpful?
   - Could commands be more intuitive?
3. **Integration gaps**
   - Is evolve loop running automatically?
   - Are all providers fully supported?
4. **Test gaps**
   - Are error cases tested?
   - Are edge cases covered?

## Output

Write findings to memory with `/learn`:
```
/learn Title of improvement found

Details about what could be better and why it matters.
```

These learnings inform the next evolution cycle's priorities.
