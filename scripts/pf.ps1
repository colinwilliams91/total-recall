param (
    [string]$Flag = "--help"
)

function Invoke-PromptFoo {
    param (
        [string]$Flag
    )
    npx --yes --registry https://registry.npmjs.org promptfoo@latest $Flag
}

# Invoke the function with the desired flag passed through as an argument
Invoke-PromptFoo -Flag $Flag