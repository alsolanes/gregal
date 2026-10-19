package xyz.solanes.gregal

import org.junit.Assert.*
import org.junit.Test

class PickerLocaleTest {
    @Test fun authoritativeCredentialStatusWinsAndUnresolvedReferencesAreNotKeys() {
        val providers = parseProviders("""{"providers":[
          {"name":"disabled","key":"sí","has_api_key":false},
          {"name":"resolved","key":"${'$'}{DUMMY_TEST_ENV}","has_api_key":true},
          {"name":"unresolved","key":"${'$'}{DUMMY_TEST_ENV}"},
          {"name":"empty","key":""},
          {"name":"blank","key":"  "},
          {"name":"legacy","key":"sí"},
          {"name":"local","url":"http://localhost:8080","key":"(buida)"}
        ]}""")
        assertEquals(listOf("resolved", "legacy"), providers.filter { it.hasKey() }.map { it.name })
    }
}
