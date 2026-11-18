# frozen_string_literal: true

# A creator's recorded LIVE sessions, following data.nextCursor page by page.
# Usage: ruby sessions.rb [HANDLE]     e.g. ruby sessions.rb tiktok
# Scope: live:read

require_relative "lib/toktokdata_http"

MAX_SESSIONS = 50
handle = (ARGV[0] || "tiktok").delete_prefix("@")

def num(value)
  value.nil? ? "-" : value.round.to_s.reverse.scan(/\d{1,3}/).join(",").reverse
end

# Yields every session, one page at a time, until nextCursor is null.
def each_session(client, handle)
  return enum_for(:each_session, client, handle) unless block_given?

  cursor = nil
  loop do
    data = client.get("/v1/live/sessions", { creatorUniqueId: handle, limit: 20, cursor: cursor })["data"]
    data["items"].each { |s| yield s }
    cursor = data["nextCursor"]
    break if cursor.nil?
  end
end

begin
  sessions = each_session(TokTokDataHttp::Client.new, handle).first(MAX_SESSIONS)
rescue StandardError => e
  TokTokDataHttp.exit_with_error(e)
end

puts "  no recorded sessions" if sessions.empty?
sessions.each do |s|
  minutes = s["durationSeconds"] ? (s["durationSeconds"] / 60.0).round.to_s : "-"
  puts format("  %s  %4s min  peak %7s viewers  %9s diamonds",
              s["startedAt"], minutes, num(s["peakViewerCount"]), num(s["totalGiftDiamonds"]))
end
