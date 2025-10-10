# Walmart Order Checker

This tool scans your Gmail for Walmart order confirmation and cancellation emails to provide you with a summary of your order history.

## Setup

To get started, simply run the application from your terminal:

```bash
go run .
```

If you're running the tool for the first time, it will detect that `credentials.json` is missing and will print a detailed, step-by-step guide to the console. Follow these instructions to create your credentials file and authorize the application.

Once the setup is complete, you can run the application again to start scanning your emails.

### Command-Line Options

You can specify the number of days to scan using the `--days` flag:

```bash
go run . --days 30
```

The first time you run the application after setting up your credentials, you will be prompted to authorize it by following a link in your browser. After authorization, a `token.json` file will be created, and the application will proceed to scan your emails.
