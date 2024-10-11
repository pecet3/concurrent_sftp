FROM mcr.microsoft.com/devcontainers/go:1-1.23-bookworm AS builder

WORKDIR /app

# Kopiowanie plików źródłowych
COPY . .

# Pobieranie zależności
RUN go mod download

# Budowanie aplikacji
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Etap finalny
FROM scratch

WORKDIR /

# Kopiowanie skompilowanej aplikacji z etapu budowania
COPY --from=builder /app/ .


# Kopiowanie certyfikatów CA
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Ustawienie zmiennej środowiskowej dla portu
ENV PORT=9000

# Wystawienie portu
EXPOSE 9000

CMD ["./main"]