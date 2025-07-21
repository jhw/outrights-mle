# Claude Development Notes

## Git Management

### Binary Files
Large Go binaries should be gitignored to avoid repository bloat:
- Built executables like `demo`, `concat_markets`, etc. are added to `.gitignore`
- Binaries are platform-specific and should be built locally via `go build`
- Use `git rm --cached <binary>` to remove from tracking if accidentally committed