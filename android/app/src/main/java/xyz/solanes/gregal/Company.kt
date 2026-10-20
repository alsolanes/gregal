package xyz.solanes.gregal

import org.json.JSONArray
import org.json.JSONObject

/**
 * Mode company (estil Dot): memòria personal persistent + check-ins.
 *
 * El backend (internal/companion, rutes /api/companion) fa la feina de model;
 * aquí només hi ha els constructors de petició i el parseig, tots testejats
 * per CompanyModeTest, i la pantalla CompanyScreen.
 */

/** Cossos de les peticions POST del mode company (contracte a docs/api-contract.md). */
object CompanionRequests {
    fun accept(fact: String): String = JSONObject().put("action", "accept").put("fact", fact).toString()
    fun dismiss(): String = JSONObject().put("action", "dismiss").toString()
    fun forget(term: String): String = JSONObject().put("action", "forget").put("term", term).toString()
}

/** Converteix una petició de Company en una tasca de l'agent del projecte actiu. */
object CompanyWorkPrompt {
    fun build(request: String, makeChanges: Boolean): String {
        val task = request.trim()
        if (task.isBlank()) return ""
        val instruction = if (makeChanges) {
            "Treballa dins del projecte actiu de Gregal. Llegeix primer les instruccions i el codi rellevant, fes els canvis demanats i executa les proves adequades. No despleguis, reiniciïs serveis ni facis accions destructives o externes; si alguna és necessària, atura't i demana aprovació explícita."
        } else {
            "Investiga el projecte actiu de Gregal en mode només lectura. Llegeix les instruccions i les fonts pertinents; no modifiquis cap fitxer ni executis accions externes. Basa les conclusions en proves observables i indica què no has pogut verificar."
        }
        return "$instruction\n\nPetició de l'usuari:\n$task"
    }
}

/** La proposta de fets del model + la memòria actual, com les envia GET /api/companion/memory. */
data class CompanyState(
    val state: String = "none",          // pending | ready | none
    val facts: List<String> = emptyList(),
    val notes: List<String> = emptyList(),
) {
    companion object {
        fun parse(raw: String): CompanyState {
            val o = JSONObject(raw)
            fun arr(a: JSONArray?): List<String> =
                a?.let { (0 until it.length()).map { i -> it.optString(i) } } ?: emptyList()
            return CompanyState(
                state = o.optString("state", "none"),
                facts = arr(o.optJSONArray("facts")),
                notes = arr(o.optJSONArray("notes")),
            )
        }
    }
}

/** Un check-in generat pel backend: text curt pensat per a una notificació. */
data class CompanyCheckin(val text: String, val at: String) {
    companion object {
        fun parse(raw: String): CompanyCheckin =
            CompanyCheckin(JSONObject(raw).optString("checkin"), JSONObject(raw).optString("at"))
    }
}
