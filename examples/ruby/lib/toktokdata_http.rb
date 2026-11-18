# frozen_string_literal: true

require "json"
require "net/http"
require "openssl"
require "uri"

# A minimal toktokdata.live client: auth, query encoding, 202 waits, retries and the error envelope.
module TokTokDataHttp
  MAX_RETRIES = 2
  PENDING_DEADLINE_SECONDS = 180

  # Raised for any 4xx/5xx answer; attributes mirror the error envelope.
  class ApiError < StandardError
    attr_reader :status, :code, :request_id, :retryable, :retry_after, :details

    def initialize(status, envelope, retry_after)
      super(envelope["message"] || "HTTP #{status}")
      @status = status
      @code = envelope["code"] || "http_error"
      @request_id = envelope["requestId"]
      @retryable = envelope["retryable"] == true
      @retry_after = retry_after # seconds, from the Retry-After header
      @details = envelope        # the whole envelope (402 carries resetAt)
    end
  end

  # Raised when a 202 "still collecting" answer did not turn into data within 180 s.
  class PendingTimeoutError < StandardError; end

  class Client
    def initialize
      @api_key = ENV.fetch("TOKTOKDATA_API_KEY", "")
      if @api_key.empty?
        raise ArgumentError, "Set TOKTOKDATA_API_KEY (dashboard Settings -> API keys at https://toktokdata.live)."
      end

      @base_url = (ENV["TOKTOKDATA_BASE_URL"] || "https://api.toktokdata.live").chomp("/")
    end

    # GET a JSON resource. Waits through 202, retries 429 and retryable 5xx.
    def get(path, query = {})
      params = query.compact.transform_values(&:to_s)
      uri = URI(@base_url + path)
      uri.query = URI.encode_www_form(params) unless params.empty?
      deadline = Time.now + PENDING_DEADLINE_SECONDS
      attempt = 0

      loop do
        response = request(uri)
        status = response.code.to_i
        retry_after = Integer(response["Retry-After"], exception: false)

        if status == 202
          # Still collecting: wait retryAfterSeconds (or Retry-After), clamped to 1..120 s, and ask again.
          pending = JSON.parse(response.body)
          wait = (pending["retryAfterSeconds"] || retry_after || 5).to_i.clamp(1, 120)
          raise PendingTimeoutError, "Still collecting after 180 s (job #{pending["jobId"]})" if Time.now + wait > deadline

          warn "  202 #{pending["status"] || "resolving"}, asking again in #{wait} s"
          sleep wait
          next
        end
        return response.body.to_s.empty? ? nil : JSON.parse(response.body) if status.between?(200, 299)

        envelope = parse_envelope(response.body.to_s)
        error = ApiError.new(status, envelope, retry_after)
        retry_it = (status == 429 && error.code != "daily_quota_exceeded") ||
                   (status >= 500 && (!envelope.key?("retryable") || error.retryable))
        raise error if !retry_it || attempt >= MAX_RETRIES

        sleep(retry_after ? [retry_after, 60].min : (0.5 * (2**attempt)) + rand(0.25))
        attempt += 1
      end
    end

    private

    def request(uri)
      req = Net::HTTP::Get.new(uri)
      req["Authorization"] = "Bearer #{@api_key}"
      req["Accept"] = "application/json"
      req["User-Agent"] = "toktokdata-ruby-example/0.1.0"
      Net::HTTP.start(uri.host, uri.port, use_ssl: uri.scheme == "https", open_timeout: 10, read_timeout: 30) do |http|
        http.request(req)
      end
    end

    def parse_envelope(body)
      parsed = JSON.parse(body)
      return parsed if parsed.is_a?(Hash) && parsed.key?("code")

      { "code" => "http_error", "message" => body[0, 200] }
    rescue JSON::ParserError
      { "code" => "http_error", "message" => body[0, 200] }
    end
  end

  class InvalidSignature < StandardError; end

  # Checks x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<x-toktokdata-timestamp>.<raw body>")>
  # and that the timestamp is within the tolerance, then returns the parsed payload.
  def self.verify_webhook(raw_body, timestamp, signature, secret, tolerance_seconds: 300)
    raise InvalidSignature, "missing signature headers" if timestamp.nil? || signature.nil?

    timestamp = timestamp.strip
    ts = Integer(timestamp, 10, exception: false) or raise InvalidSignature, "bad timestamp"
    raise InvalidSignature, "timestamp outside the tolerance" if (Time.now.to_i - ts).abs > tolerance_seconds

    expected = "sha256=#{OpenSSL::HMAC.hexdigest("SHA256", secret, "#{timestamp}.#{raw_body}")}"
    # OpenSSL.secure_compare is constant-time.
    raise InvalidSignature, "signature mismatch" unless OpenSSL.secure_compare(expected, signature.strip)

    JSON.parse(raw_body)
  rescue JSON::ParserError
    raise InvalidSignature, "body is not JSON"
  end

  # Prints an API error the way a user can act on it and exits non-zero.
  def self.exit_with_error(error)
    case error
    when ApiError
      warn "HTTP #{error.status} #{error.code}: #{error.message} (requestId #{error.request_id || "-"})"
      case error.status
      when 402 then warn "Credits are spent; they reset at #{error.details["resetAt"] || "?"}."
      when 429 then warn "Rate limited; retry after #{error.retry_after || "a few"} s."
      when 403 then warn "The key lacks the scope (insufficient_scope) or the plan lacks the feature (module_not_entitled)."
      end
    when PendingTimeoutError
      warn "#{error.message}; try again in a minute."
    else
      warn error.message
    end
    exit 1
  end
end
