package xyz.solanes.gregal

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class ChatFollowDeviceTest {
    @get:Rule val ui = createComposeRule()

    @Test fun tallMessageReadBackSendResizeAndReturnToBottom() {
        lateinit var list: LazyListState
        lateinit var follow: ChatFollowState
        var count by mutableIntStateOf(40)
        var height by mutableIntStateOf(500)
        var tailHeight by mutableIntStateOf(1600)
        ui.setContent {
            list = rememberLazyListState()
            follow = rememberChatFollow(list, "fixture")
            LazyColumn(Modifier.height(height.dp).fillMaxWidth().testTag("list"), state = list) {
                items(count) { i -> Text("message $i", Modifier.height(72.dp)) }
                item { Text("long tail", Modifier.height(tailHeight.dp)) }
            }
        }
        ui.waitForIdle()
        ui.runOnIdle { assertFalse("initial tall tail bottom", list.canScrollForward) }
        ui.onNodeWithTag("list").performTouchInput { swipeDown() }
        ui.waitForIdle()
        val previous = list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset
        ui.runOnIdle { assertFalse("reading history disables follow", follow.following); tailHeight += 200 }
        ui.waitForIdle()
        ui.runOnIdle { assertEquals("new content must not yank a reader", previous, list.firstVisibleItemIndex to list.firstVisibleItemScrollOffset) }
        ui.runOnIdle { follow.requestFollow(); count++ }
        ui.waitForIdle()
        ui.runOnIdle { assertFalse("send reaches the bottom", list.canScrollForward); height = 250 }
        ui.waitForIdle()
        ui.runOnIdle { assertFalse("keyboard resize keeps the bottom visible", list.canScrollForward) }
        ui.onNodeWithTag("list").performTouchInput { swipeDown() }
        ui.waitForIdle()
        ui.runOnIdle { assertFalse(follow.following) }
        repeat(4) { ui.onNodeWithTag("list").performTouchInput { swipeUp() }; ui.waitForIdle() }
        ui.runOnIdle { assertFalse("fling reaches bottom", list.canScrollForward); assertTrue("following resumes after fling", follow.following) }
        ui.runOnIdle { tailHeight += 200 }
        ui.waitForIdle()
        ui.runOnIdle { assertFalse("streamed growth is followed", list.canScrollForward) }
    }
}
