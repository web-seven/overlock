# Contributing to Overlock

We welcome contributions to Overlock! Whether you're fixing a bug, improving documentation, or adding new features, we are excited to work with you. Please take a moment to read through the following guidelines to make the contribution process easy and effective for everyone involved.

## How to Contribute

### 1. Fork the Repository

Start by forking the repository to your GitHub account:

1. Go to the Overlock repository on GitHub.
2. Click the "Fork" button in the upper right corner.
3. Clone the forked repository to your local machine:

   ```bash
   git clone https://github.com/web-seven/overlock.git
   cd overlock
   ```

### 2. Create a Branch

We follow the Git branching model to manage development. Use a descriptive name for your branch to indicate the purpose of the work:

```bash
git checkout -b feature/your-feature-name
```

For bug fixes, use the prefix `fix/`:

```bash
git checkout -b fix/your-bug-fix
```

### 3. Make Your Changes

Make your changes to the codebase. Please ensure that:

- Your code follows the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) guidelines.
- You write unit tests for any new functionality.
- The code compiles without errors (`go build`).
- Run the tests to make sure everything is working as expected:

  ```bash
  go test ./...
  ```

### 4. Commit Your Changes

Make sure to write clear and concise commit messages. Follow the conventional commit format:

- **feat:** A new feature
- **fix:** A bug fix
- **docs:** Documentation changes
- **style:** Code style changes (formatting, missing semicolons, etc.)
- **refactor:** Code refactoring without changing functionality
- **test:** Adding or updating tests
- **chore:** Updating build tasks, package manager configs, etc.

Example:

```bash
git commit -m "feat: add support for multiple clusters"
```

### 5. Push to Your Branch

Push your changes to your forked repository:

```bash
git push origin feature/your-feature-name
```

### 6. Create a Pull Request

Go to the original repository and create a pull request (PR) from your fork:

1. Click on "Pull requests" in the original repository.
2. Click "New pull request."
3. Choose your forked repository and branch, and create the pull request.
4. Write a meaningful description of the changes in the PR, and link any relevant issues.

### 7. Review Process

One of the maintainers will review your PR and may suggest changes or improvements. After addressing the comments, push the updates to your branch and the PR will be updated automatically.

Please be patient as reviews can take time depending on the maintainers' availability.

## Coding Guidelines

- Follow idiomatic Go programming practices. You can refer to the [Effective Go](https://golang.org/doc/effective_go.html) documentation for best practices.
- Ensure your code is formatted with `gofmt`.
- Use `go vet` to catch common errors.
- Make sure all code is properly documented using Go's commenting conventions.

## Reporting Issues

If you find a bug or have a feature request, please open an issue in the GitHub repository. When reporting an issue, include:

- A clear and descriptive title.
- A detailed description of the issue or the requested feature.
- Steps to reproduce the issue, if applicable.
- The environment in which the issue occurs (e.g., OS, Go version).

## License

By contributing to Overlock, you agree that your contributions will be licensed under the MIT License.
