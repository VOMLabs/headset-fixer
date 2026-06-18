#!/usr/bin/env bash

echo "🧹 Cleaning Android + PATH environment..."

# ---- Define clean base PATH (we rebuild from scratch) ----
CLEAN_PATHS=(
"$HOME/.local/bin"
"$HOME/.sdkman/candidates/kotlin/current/bin"
"$HOME/.sdkman/candidates/java/current/bin"
"$HOME/.bun/bin"
"/usr/local/bin"
"/usr/bin"
"/bin"
"/usr/local/sbin"
"/usr/lib/jvm/default/bin"
"$HOME/.local/share/JetBrains/Toolbox/scripts"
"$HOME/.spicetify"
"$HOME/Android/Sdk/platform-tools"
"$HOME/Android/Sdk/emulator"
"$HOME/Android/Sdk/cmdline-tools/latest/bin"
)

# ---- rebuild PATH ----
NEW_PATH=""
for p in "${CLEAN_PATHS[@]}"; do
    if [ -d "$p" ]; then
        NEW_PATH="$NEW_PATH:$p"
    fi
done

# remove leading colon
NEW_PATH="${NEW_PATH#:}"

export PATH="$NEW_PATH"

echo "✅ PATH cleaned for current session"
echo "----------------------------------"
echo "$PATH" | tr ':' '\n'

# ---- optional cleanup of old Android SDK leftovers ----
echo ""
echo "🧽 Removing old /opt/android-sdk entries from PATH if present..."

export PATH="$(echo "$PATH" | tr ':' '\n' | grep -v '^/opt/android-sdk' | awk '!seen[$0]++' | tr '\n' ':' | sed 's/:$//')"

echo "✅ Final PATH:"
echo "$PATH" | tr ':' '\n'

echo ""
echo "🎉 Done. Restart terminal for permanent clean state."
