package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

/** Authenticated profile. Topic knowledge and personal memory are distinct stores. */
data class WindProfile(val wind: Wind, val knowledge: String, val personalMemory: String) {
    companion object {
        fun parse(raw: String): WindProfile {
            val obj = JSONObject(raw)
            val wind = obj.getJSONObject("wind")
            val parsed = Wind.parse(JSONObject().put("winds", JSONArray().put(wind)).toString()).first()
            return WindProfile(parsed, obj.optString("knowledge"), obj.optString("personal_memory"))
        }
    }
}
