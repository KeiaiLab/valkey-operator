# Contributing

Thanks for your interest in contributing to `valkey-operator`! GitHub is the
canonical repository — please open Pull Requests here. (Any GitLab copy is a
read-only archive mirror, not a place to send changes.)

For the full contributing guide (prerequisites, PR workflow, ADR policy,
code style, and quality gates), see
[.github/CONTRIBUTING.md](.github/CONTRIBUTING.md).

## Development workflow

1. Create a feature branch: `git checkout -b feat/<topic>`.
2. Make your change, with tests for any behavioral difference.
3. Make sure lint and tests pass locally.
4. Open a Pull Request against `main`.

## Guidelines

- Follow the existing code style of the project.
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit
  messages (`feat:`, `fix:`, `docs:`, `chore:` …).
- Keep changes focused and atomic — one logical change per Pull Request.
- Write or update tests; a behavioral change without a test is incomplete.

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
