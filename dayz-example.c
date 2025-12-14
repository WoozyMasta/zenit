#ifdef SERVER
class SomeClass
{
  static const string MOD_NAME = "MySupperMode";
  static const string MOD_VERSION = "1.2.5";
  static const string TELEMETRY_URL = "https://zenit.woozymasta.ru";
	static const int TELEMETRY_DELAY = 600000; // 10-20 min
  protected static bool s_TelemetrySend;

	/**
	    \brief Finalizes the loading process.
	*/
	void OnLoad()
	{
    // Some code

#ifndef DIAG
		if (!disableTelemetry && !s_TelemetrySend) {
			s_TelemetrySend = true;
			int delay = Math.RandomInt(TELEMETRY_DELAY, TELEMETRY_DELAY * 2);
			g_Game.GetCallQueue(CALL_CATEGORY_SYSTEM).CallLater(SendTelemetry, delay, false);
		}
#endif
	}

	/**
	    \brief Telemetry sender
	*/
	protected void SendTelemetry()
	{
		RestApi api = GetRestApi();
		if (!api)
			api = CreateRestApi();
		if (!api) {
			s_TelemetrySend = false;
			return;
		}

		RestContext ctx = api.GetRestContext(TELEMETRY_URL);
		if (!ctx) {
			s_TelemetrySend = false;
			return;
		}

		string body = string.Format(
		                  "{\"application\":\"%1\",\"version\":\"%2\",\"type\":\"steam\",\"port\":%3}",
		                  MOD_NAME, MOD_VERSION, g_Game.ServerConfigGetInt("steamQueryPort"));

		ctx.SetHeader("application/json");
		ctx.POST(null, "/api/telemetry", body);
	}
}
#endif
