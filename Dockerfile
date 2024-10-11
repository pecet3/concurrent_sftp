FROM mcr.microsoft.com/devcontainers/go:1-1.23-bookworm

WORKDIR /app

# Kopiowanie plików źródłowych
COPY . .

# Pobieranie zależności
RUN go mod download

# Budowanie aplikacji
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Etap finalny


# Kopiowanie pliku .env
COPY .env .

# Instalacja certyfikatów CA (często wymagane dla HTTPS)
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Ustawienie zmiennej środowiskowej dla portu
ENV PORT=9000

# Wystawienie portu
EXPOSE 9000

# Uruchomienie aplikacji
CMD ["./main"]