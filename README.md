# Node Version Selector (NVS)

NVS automatically determines and executes only the main Node version, but NVS does not manage tools by versions.

## Install

```
go install github.com/komem3/nvs@latest

nvs init

# Add PATH
# export PATH="$HOME/.nvs/bin:$PATH"

nvs use 20
node --version
```

## Version Determination

1. read `.node-version` in current path.
2. read `engines` field of `package.json` in current path.
3. go to the parent directory. Back to 1. If there are no more parents, Go to 4.
4. read global version file(`$HOME/.nvs/version`)

## Install Global Tool

If you want to install a tool in a global version instead of a local version,
you can use the global version with the following command.

```
nvs install prettier
```
