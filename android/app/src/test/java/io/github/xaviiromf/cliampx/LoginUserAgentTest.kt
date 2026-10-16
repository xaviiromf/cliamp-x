package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class LoginUserAgentTest {
    private val webView = "Mozilla/5.0 (Linux; Android 11; moto g(20) Build/RTAS31.68-66-3; wv) " +
        "AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/141.0.7390.122 Mobile Safari/537.36"

    @Test fun usesTheDeviceAndTheWebViewChrome() {
        val ua = LoginActivity.chromeUserAgent(webView, "11", "moto g(20)")
        assertEquals(
            "Mozilla/5.0 (Linux; Android 11; moto g 20) AppleWebKit/537.36 (KHTML, like Gecko) " +
                "Chrome/141.0.7390.122 Mobile Safari/537.36",
            ua,
        )
        assertFalse("no WebView token", ua.contains("wv"))
    }

    @Test fun fallsBackWhenTheDefaultHasNoChrome() {
        assertEquals(
            "Mozilla/5.0 (Linux; Android 16; SM-A556E) AppleWebKit/537.36 (KHTML, like Gecko) " +
                "Chrome/141.0.0.0 Mobile Safari/537.36",
            LoginActivity.chromeUserAgent("", "16", "SM-A556E"),
        )
    }

    @Test fun anEmptyModelBecomesK() {
        assertEquals(true, LoginActivity.chromeUserAgent(webView, "14", " ").contains("Android 14; K)"))
    }
}
