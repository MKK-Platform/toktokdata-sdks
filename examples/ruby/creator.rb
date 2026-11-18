# frozen_string_literal: true

# A creator profile. A creator not stored yet answers 202 "resolving"; the client waits and asks again.
# Usage: ruby creator.rb [HANDLE]     e.g. ruby creator.rb tiktok
# Scope: creator:read

require "erb"
require_relative "lib/toktokdata_http"

handle = (ARGV[0] || "tiktok").delete_prefix("@")

def num(value)
  value.nil? ? "-" : value.round.to_s.reverse.scan(/\d{1,3}/).join(",").reverse
end

begin
  c = TokTokDataHttp::Client.new.get("/v1/creators/#{ERB::Util.url_encode(handle)}")["data"]["creator"]
rescue StandardError => e
  TokTokDataHttp.exit_with_error(e)
end

puts "#{c["displayName"] || c["uniqueId"]} (@#{c["uniqueId"]})#{c["verified"] ? " [verified]" : ""}"
puts "  region #{c["region"] || "?"}, #{num(c["followerCount"])} followers, " \
     "#{num(c["awemeCount"])} videos, #{num(c["totalFavorited"])} likes"
puts "  last refreshed #{c["lastRefreshedAt"] || "-"}"
