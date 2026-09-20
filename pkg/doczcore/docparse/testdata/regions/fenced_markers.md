# Markers inside fences

A document that documents markers must not be parsed as if it used them.

```markdown
<!--docz:tasks:start-->
- [ ] this is an example, not a task region
<!--docz:tasks:end-->
```

Prose between the fences.

~~~
<!--docz:summary:start-->
~~~

A tilde fence does not toggle, so the marker above is real and unclosed
until the one below closes it.

<!--docz:summary:end-->

<!--docz:references:start-->
## References

- A real region after all that.
<!--docz:references:end-->
