# kotlinx.serialization generated serializers.
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.**
-keepclassmembers @kotlinx.serialization.Serializable class ** {
    *** Companion;
    *** INSTANCE;
    kotlinx.serialization.KSerializer serializer(...);
}
-keep class com.deranjer.cookira.data.** { *; }

# Retrofit
-keepattributes Signature, Exceptions
-dontwarn retrofit2.**
-keep,allowobfuscation interface <1>

# OkHttp
-dontwarn okhttp3.**
-dontwarn okio.**
