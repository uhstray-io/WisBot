# syntax=docker/dockerfile:1

# ── Build ─────────────────────────────────────────────────────────────────────
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src

# Restore first (cached layer) — only the project file invalidates it.
COPY Wisbot.csproj ./
RUN dotnet restore Wisbot.csproj -r linux-x64

COPY . .
RUN dotnet publish Wisbot.csproj -c Release -r linux-x64 --no-self-contained -o /app

# ── Runtime ───────────────────────────────────────────────────────────────────
FROM mcr.microsoft.com/dotnet/runtime:10.0 AS runtime

# Native libs for Discord voice. libsodium.so + libe_sqlite3.so ship via NuGet
# (version-matched) and land in /app, so no apt libsodium is needed. opus is NOT
# NuGet-provided for Linux, so install libopus0 and symlink the unversioned name
# Discord.Net's DllImport("opus") probes. curl is for the orchestration health check.
RUN apt-get update \
    && apt-get install -y --no-install-recommends libopus0 curl \
    && ln -sf /usr/lib/x86_64-linux-gnu/libopus.so.0 /usr/lib/x86_64-linux-gnu/libopus.so \
    && rm -rf /var/lib/apt/lists/*

# Run as non-root.
RUN useradd --uid 10001 --create-home wisbot

WORKDIR /app
COPY --from=build /app ./

# Mount points for persistence (compose mounts named volumes here).
RUN mkdir -p /app/data /app/recordings && chown -R wisbot:wisbot /app
USER wisbot

# Container defaults — bind health on all interfaces so Docker port mapping reaches it,
# and point persistence at the mounted volumes. Secrets/IDs come from the env file.
ENV WISBOT_HEALTH_HOST="+" \
    WISBOT_HEALTH_PORT=8080 \
    WISBOT_DB_PATH=/app/data/wisbot.db \
    WISBOT_RECORDINGS_DIR=/app/recordings

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -sf "http://localhost:8080/health" || exit 1

ENTRYPOINT ["dotnet", "Wisbot.dll"]
