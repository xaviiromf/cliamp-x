package io.github.xaviiromf.cliampx

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.w3c.dom.Element
import java.io.File
import javax.xml.parsers.DocumentBuilderFactory

/** English (values) and Spanish (values-es) name the same texts with the same format arguments (FINAL-PLAN T9). */
class StringsTest {
    private val res = File("src/main/res")

    /** name → the sorted format arguments of every translatable string and plural in dir. */
    private fun texts(dir: String): Map<String, List<String>> {
        val out = HashMap<String, List<String>>()
        val files = File(res, dir).listFiles { f -> f.name.startsWith("strings") && f.name.endsWith(".xml") }.orEmpty()
        assertTrue("no strings in $dir", files.isNotEmpty())
        for (file in files) {
            val root = DocumentBuilderFactory.newInstance().newDocumentBuilder().parse(file).documentElement
            for (tag in listOf("string", "plurals")) {
                val nodes = root.getElementsByTagName(tag)
                for (i in 0 until nodes.length) {
                    val e = nodes.item(i) as Element
                    if (e.getAttribute("translatable") == "false") continue
                    val args = FORMAT.findAll(e.textContent).map { it.value }.toSortedSet().toList()
                    assertTrue("$dir/${file.name}: ${e.getAttribute("name")} twice", out.put(e.getAttribute("name"), args) == null)
                }
            }
        }
        return out
    }

    @Test
    fun spanishMatchesEnglish() {
        val en = texts("values")
        val es = texts("values-es")
        assertEquals("names only in English", emptySet<String>(), en.keys - es.keys)
        assertEquals("names only in Spanish", emptySet<String>(), es.keys - en.keys)
        for ((name, args) in en) assertEquals("format arguments of $name", args, es[name])
    }

    private companion object {
        val FORMAT = Regex("%\\d+\\$[sd]")
    }
}

class LanguageTest {
    @Test
    fun withRegion() {
        val chile = java.util.Locale.Builder().setLanguage("es").setRegion("CL").build()
        val usEnglish = java.util.Locale.Builder().setLanguage("en").setRegion("US").build()
        for ((app, want) in listOf(
            null to chile, // the system's
            java.util.Locale.Builder().setLanguage("en").build() to java.util.Locale.Builder().setLanguage("en").setRegion("CL").build(),
            usEnglish to usEnglish, // a region of its own stays
        )) assertEquals(want, Language.withRegion(app, chile))
    }
}
