# frozen_string_literal: true

# Top 10 of an official TikTok LIVE board for one region.
# Usage: ruby leaderboard.rb [REGION] [BOARD]     e.g. ruby leaderboard.rb VN hourly
# Scope: rank:read

require_relative "lib/toktokdata_http"

region = ARGV[0] || "VN"
board_key = ARGV[1] || "hourly"

begin
  res = TokTokDataHttp::Client.new.get("/v1/rankings/official", { board: board_key, region: region, limit: 10 })
rescue StandardError => e
  TokTokDataHttp.exit_with_error(e)
end

board = res["data"]["board"]
puts "#{board["board"]} board, #{board["region"]}, period #{board["periodKey"]}"
unless board["publishable"]
  # Coverage below the gate: the order may still change, so label it as partial.
  cov = board["coverage"]
  puts format("(partial: coverage %.0f%% of the %.0f%% gate)", cov["ratio"] * 100, cov["threshold"] * 100)
end

board["entries"].each do |e|
  who = e["masked"] ? "(masked, plan #{e["unmaskTier"]})" : "#{e["nickname"]} @#{e["uniqueId"]}"
  score = e["score"].round.to_s.reverse.scan(/\d{1,3}/).join(",").reverse
  puts format("%3d  %-40s %12s%s", e["rank"], who, score, e["isLive"] ? "  LIVE" : "")
end
