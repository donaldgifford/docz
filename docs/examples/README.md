# Tools Directory - DEPRECATED

**⚠️ WARNING: These bash scripts are DEPRECATED and will be removed in v2.0**

## Migration to Rex CLI

All bash scripts in this directory have been replaced by the **rex CLI tool**.

### Quick Migration Guide

| Old Bash Script | New Rex Command |
|----------------|-----------------|
| `tools/docs/create-adr.sh "Title"` | `rex adr create "Title"` |
| `tools/docs/create-rfc.sh "Title"` | `rex rfc create "Title"` |
| `tools/docs/update-adr-readme.sh` | `rex adr update` |
| `tools/docs/update-rfc-readme.sh` | `rex rfc update` |
| `tools/scripts/tasks/task-create.sh` | `rex task create` |
| `tools/scripts/tasks/task-list.sh` | `rex task list` |
| `tools/scripts/tasks/task-stats.sh` | `rex task stats` |
| `tools/scripts/tasks/task-complete.sh TASK-001` | `rex task complete TASK-001` |
| `tools/scripts/tasks/task-update.sh` | `rex task update` |

### Why Migrate?

The rex CLI offers:

- **Faster**: SQLite cache provides < 100ms queries
- **No Dependencies**: Single binary, no bash/yq/mise required
- **Rich Filtering**: Filter tasks by type, status, priority, tags
- **Better UX**: Colored output, progress bars, interactive prompts
- **Cross-Platform**: Works on Linux, macOS, Windows
- **Statistics**: Comprehensive stats and progress tracking

### Installation

```bash
# Install from source (recommended during development)
cd /path/to/rex
go build -o rex main.go
sudo mv rex /usr/local/bin/

# Or via go install (once released)
go install github.com/donaldgifford/rex@latest

# Verify installation
rex version
```

### First Steps

1. **Build SQLite cache** from existing markdown files:
   ```bash
   rex rebuild
   ```

2. **Test new commands**:
   ```bash
   # List ADRs
   rex adr list

   # List tasks
   rex task list

   # View statistics
   rex task stats
   ```

3. **Update your workflow**:
   ```bash
   # Old
   make task

   # New
   rex task create
   ```

### Full Migration Guide

See [docs/MIGRATION.md](../docs/MIGRATION.md) for comprehensive migration instructions.

### Documentation

- **README.md**: Quick start and features
- **docs/COMMANDS.md**: Complete command reference
- **docs/MIGRATION.md**: Migration guide from bash scripts
- **CONTRIBUTING.md**: Contribution guidelines

### Timeline

- **Now**: Bash scripts show deprecation warnings
- **v1.x**: Both systems work in parallel
- **v2.0**: Bash scripts removed entirely

### Need Help?

- Open an issue: [rex/issues](https://github.com/donaldgifford/rex/issues)
- Check docs: [docs/](../docs/)
- Read migration guide: [MIGRATION.md](../docs/MIGRATION.md)

---

**Please migrate to rex CLI as soon as possible. These bash scripts will be removed in v2.0.**
