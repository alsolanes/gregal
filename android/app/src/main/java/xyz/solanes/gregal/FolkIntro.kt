package xyz.solanes.gregal

import android.graphics.ImageDecoder
import android.graphics.drawable.Animatable2
import android.graphics.drawable.AnimatedImageDrawable
import android.graphics.drawable.Drawable
import android.os.Build
import android.widget.ImageView
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material3.Button
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.text.style.TextAlign
import kotlinx.coroutines.launch
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.verticalScroll

/**
 * The welcome of the server setup (PairingScreen): the felt winds hop in,
 * gather round a little server whose light turns green, cheer, and the Gregal
 * mark rises behind them (tools/characters/intro.py, res/raw/intro_folk.webp).
 * It plays once each time the setup opens (first run or reconfiguring); a
 * still of its last frame stands in without motion, before Android 9, or once
 * it has played. [onDone] tells the screen to bring in the form.
 */
@Composable
internal fun FolkIntro(lang: String, play: Boolean, onDone: () -> Unit, modifier: Modifier = Modifier) {
    val done by rememberUpdatedState(onDone)
    var finished by remember { mutableStateOf(!play || Build.VERSION.SDK_INT < Build.VERSION_CODES.P) }
    var view by remember { mutableStateOf<ImageView?>(null) }
    fun showEnd() {
        view?.setImageResource(R.drawable.intro_folk_end)
        if (!finished) {
            finished = true
            done()
        }
    }
    Box(modifier.fillMaxWidth().aspectRatio(1f).testTag("folk-intro"), contentAlignment = Alignment.BottomEnd) {
        AndroidView(
            factory = { ctx ->
                ImageView(ctx).apply {
                    scaleType = ImageView.ScaleType.FIT_CENTER
                    contentDescription = null
                    importantForAccessibility = ImageView.IMPORTANT_FOR_ACCESSIBILITY_NO
                    view = this
                    if (finished) setImageResource(R.drawable.intro_folk_end)
                    else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                        val anim = runCatching { ImageDecoder.decodeDrawable(ImageDecoder.createSource(ctx.resources, R.raw.intro_folk)) }.getOrNull()
                        if (anim is AnimatedImageDrawable) {
                            anim.repeatCount = 0 // once
                            anim.registerAnimationCallback(object : Animatable2.AnimationCallback() {
                                override fun onAnimationEnd(drawable: Drawable?) { post { showEnd() } }
                            })
                            setImageDrawable(anim)
                            anim.start()
                        } else post { showEnd() }
                    }
                }
            },
            modifier = Modifier.fillMaxWidth().aspectRatio(1f),
        )
        if (!finished) TextButton(onClick = { showEnd() }, modifier = Modifier.padding(4.dp).testTag("folk-intro-skip")) {
            Text(tr(lang, "app.intro.skip"), color = Muted, fontSize = GregalSize.small)
        }
    }
    DisposableEffect(Unit) {
        if (finished) done()
        onDispose { (view?.drawable as? AnimatedImageDrawable)?.stop() }
    }
}

/** A felt character saying something in a bubble: how the setup asks for each thing. */
@Composable
internal fun FolkSays(who: String, text: String, modifier: Modifier = Modifier) {
    Row(modifier.fillMaxWidth().testTag("folk-says-$who"), verticalAlignment = Alignment.Bottom, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        FolkPortrait(who, 52)
        Surface(shape = RoundedCornerShape(18.dp, 18.dp, 18.dp, 6.dp), color = SeaSoft(0.14f), modifier = Modifier.weight(1f)) {
            Text(text, color = Ink, style = GregalType.bodySmall, modifier = Modifier.padding(horizontal = 14.dp, vertical = 10.dp))
        }
        Spacer(Modifier.width(4.dp))
    }
}

/**
 * The first time the app opens: the 3D welcome and the tour, full screen over
 * whatever is behind (the app may already have connected on its own). The
 * server setup (PairingScreen) plays the same welcome when it is opened later.
 */
@Composable
internal fun FolkWelcome(lang: String, onDone: () -> Unit) {
    var stage by androidx.compose.runtime.saveable.rememberSaveable { androidx.compose.runtime.mutableIntStateOf(0) }
    androidx.activity.compose.BackHandler(onBack = onDone)
    GregalBg {
        Column(Modifier.fillMaxWidth().fillMaxHeight().verticalScroll(androidx.compose.foundation.rememberScrollState())
            .padding(horizontal = 28.dp, vertical = 36.dp).testTag("folk-welcome"),
            horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            FolkIntro(lang, play = stage == 0 && windMotionEnabled(), onDone = { if (stage == 0) stage = 1 },
                modifier = Modifier.widthIn(max = if (stage == 0) 380.dp else 240.dp))
            if (stage == 1) FolkTour(lang, onDone = onDone, modifier = Modifier.widthIn(max = 460.dp))
        }
    }
}
