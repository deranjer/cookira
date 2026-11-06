package com.deranjer.cookira

import com.deranjer.cookira.ui.recipes.DurationParser
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class DurationParserTest {
    private fun secs(text: String) = DurationParser.find(text).map { it.seconds }

    @Test fun `range starts at the low end`() {
        val d = DurationParser.find("Cover and cook on LOW approximately 4–5 hours, until tender.").single()
        assertEquals(4 * 3600, d.seconds)
        assertEquals("4–5 hours", d.text)
        assertTrue(d.isRange)
    }

    @Test fun `minutes and fractions`() {
        assertEquals(listOf(180), secs("broil for 3–5 minutes to brown"))
        assertEquals(listOf(5400), secs("simmer 1½ hours"))
        assertEquals(listOf(1800), secs("rest ½ hour"))
        assertEquals(listOf(90), secs("whisk 90 sec"))
    }

    @Test fun `several in one step, deduplicated`() {
        assertEquals(listOf(600, 1200), secs("Bake 10 minutes, flip, then 20 min more. Cool 10 minutes."))
    }

    @Test fun `ignores things that are not times`() {
        assertTrue(secs("Add 1 tbsp minced garlic and 2 cups water").isEmpty())
        assertTrue(secs("Heat to 165°F internally. Use the second pan.").isEmpty())
    }

    @Test fun `clock format`() {
        assertEquals("3:25", DurationParser.clock(205))
        assertEquals("1:02:05", DurationParser.clock(3725))
        assertEquals("0:00", DurationParser.clock(-4))
    }
}
