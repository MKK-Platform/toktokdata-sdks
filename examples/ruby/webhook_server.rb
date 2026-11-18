# frozen_string_literal: true

# Receives toktokdata.live alert webhooks and verifies their signature.
# Usage: TOKTOKDATA_WEBHOOK_SECRET=... ruby webhook_server.rb [PORT]
# No API key needed here. Ruby 3 no longer bundles WEBrick, so this is a tiny stdlib socket
# server that is enough for local testing; in an app, call TokTokDataHttp.verify_webhook from
# your Rack/Rails/Sinatra handler with the raw request body.

require "socket"
require_relative "lib/toktokdata_http"

secret = ENV.fetch("TOKTOKDATA_WEBHOOK_SECRET", "")
abort "Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel." if secret.empty?
port = Integer(ARGV[0] || "8080")

def respond(client, status, reason)
  client.write("HTTP/1.1 #{status} #{reason}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
end

server = TCPServer.new("127.0.0.1", port)
puts "listening on http://localhost:#{port}/ (POST only)"

loop do
  client = server.accept
  begin
    method = client.gets.to_s.split(" ").first
    headers = {}
    while (line = client.gets) && line != "\r\n"
      name, value = line.split(":", 2)
      headers[name.strip.downcase] = value.to_s.strip
    end
    next respond(client, 405, "Method Not Allowed") unless method == "POST"

    # Verify the exact bytes that were signed.
    raw_body = client.read(headers["content-length"].to_i).to_s
    payload = TokTokDataHttp.verify_webhook(raw_body, headers["x-toktokdata-timestamp"],
                                            headers["x-toktokdata-signature"], secret)
    puts "alert #{payload["alertId"]} from rule \"#{payload.dig("rule", "name")}\" " \
         "at #{payload["occurredAt"] || payload["deliveredAt"]}"
    puts JSON.pretty_generate(payload["event"])
    respond(client, 204, "No Content")
  rescue TokTokDataHttp::InvalidSignature => e
    warn "rejected webhook: #{e.message}"
    respond(client, 401, "Unauthorized")
  ensure
    client.close
  end
end
