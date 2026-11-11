package xyz.solanes.gregal

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonClassDiscriminator

/*
 * Contracte compartit de l'assistent (pla v2, §9): els objectes que el
 * servidor i el mòbil s'envien. Els vocabularis d'estat són els de
 * internal/web/assistant_contract.go. Tots els camps tenen valor per defecte:
 * un servidor més antic que no n'enviï algun no trenca res. Les fixtures de
 * docs/assistant-contract/fixtures es llegeixen a AssistantContractTest.
 */

@OptIn(kotlinx.serialization.ExperimentalSerializationApi::class)
@Serializable
@JsonClassDiscriminator("kind")
sealed interface AssistantObject {
    val id: String
}

/** Àmbit que l'usuari ha donat a un encàrrec: què pot llegir, preparar i fer. */
@Serializable
data class AllowedScope(
    val read: List<String> = emptyList(),
    val prepare: List<String> = emptyList(),
    val act: List<String> = emptyList(),
)

/** Un encàrrec: feina llarga amb objectiu, estat i resultats. */
@Serializable
@SerialName("encarrec")
data class Encarrec(
    override val id: String = "",
    val owner: String = "",
    val conversationId: String = "",
    val runId: Long = 0L,
    val objective: String = "",
    /** queued | running | waiting_user | waiting_connection | waiting_computer | completed | failed | cancelled */
    val state: String = "",
    val allowedScope: AllowedScope = AllowedScope(),
    val resultRefs: List<String> = emptyList(),
    val pendingDecisionId: String = "",
    val createdAt: String = "",
    val updatedAt: String = "",
) : AssistantObject

/** L'efecte que una decisió aprovaria (una revisió concreta). */
@Serializable
data class ProposedEffect(
    val type: String = "",
    val target: String = "",
    val summary: String = "",
    val draftRef: String = "",
)

/** Una decisió pendent de l'usuari sobre una revisió concreta d'un efecte. */
@Serializable
@SerialName("decisio")
data class Decisio(
    override val id: String = "",
    val owner: String = "",
    val runId: Long = 0L,
    val messageId: String = "",
    val proposedEffect: ProposedEffect = ProposedEffect(),
    val revision: Int = 0,
    /** pending | approved | rejected | expired | superseded */
    val state: String = "",
    val expiresAt: String = "",
) : AssistantObject

/** Un compte connectat (correu, calendari…). */
@Serializable
@SerialName("connexio")
data class Connexio(
    override val id: String = "",
    val owner: String = "",
    val method: String = "",
    val service: String = "",
    val accountLabel: String = "",
    /** available | connecting | connected | needs_session | error | disconnected */
    val status: String = "",
    val capabilities: List<String> = emptyList(),
    /** Referència opaca («auth:…»), mai un secret. */
    val authRef: String = "",
    val lastCheckedAt: String = "",
    val error: String = "",
) : AssistantObject

/** L'ordinador personal de l'usuari al servidor. */
@Serializable
@SerialName("ordinador")
data class Ordinador(
    override val id: String = "",
    val owner: String = "",
    /** starting | running | stopped | unreachable */
    val health: String = "",
    val capabilities: List<String> = emptyList(),
    /** agent | user | none */
    val controlOwner: String = "",
    val leaseExpiry: String = "",
    val lastSeenAt: String = "",
) : AssistantObject

/** Un missatge del fil (també els proactius). */
@Serializable
@SerialName("missatge")
data class Missatge(
    override val id: String = "",
    val conversationId: String = "",
    val source: String = "",
    val relatedRunId: Long = 0L,
    val resultRefs: List<String> = emptyList(),
    val text: String = "",
    val createdAt: String = "",
) : AssistantObject

object AssistantContract {
    fun parse(raw: String): AssistantObject = GregalJson.decodeFromString(AssistantObject.serializer(), raw)
}
