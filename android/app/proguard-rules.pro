# R8 per velocitat i mida, no per amagar res: els noms es queden tal qual
# perquè els informes d'error es llegeixin sense mapping.
-dontobfuscate

# Les classes que viatgen dins un Intent (Plots) han de mantenir la forma.
-keepclassmembers class * implements java.io.Serializable {
    static final long serialVersionUID;
    private void writeObject(java.io.ObjectOutputStream);
    private void readObject(java.io.ObjectInputStream);
    java.lang.Object writeReplace();
    java.lang.Object readResolve();
}

# kotlinx.serialization: els models @Serializable (respostes del servidor,
# contracte de l'assistent i còpies desades) es troben pel serializer() que
# genera el plugin. La llibreria ja porta regles pròpies; aquestes asseguren
# que R8 no s'endú els companion ni els $serializer dels nostres models.
-keepattributes RuntimeVisibleAnnotations,AnnotationDefault,InnerClasses
-if @kotlinx.serialization.Serializable class xyz.solanes.gregal.**
-keepclassmembers class xyz.solanes.gregal.<1> {
    static <1>$Companion Companion;
}
-if @kotlinx.serialization.Serializable class xyz.solanes.gregal.** {
    static **$* *;
}
-keepclassmembers class xyz.solanes.gregal.<2>$<3> {
    kotlinx.serialization.KSerializer serializer(...);
}
-keep,includedescriptorclasses class xyz.solanes.gregal.**$$serializer { *; }
-keepclasseswithmembers class xyz.solanes.gregal.** {
    kotlinx.serialization.KSerializer serializer(...);
}

# OkHttp porta les seves regles; aquestes classes opcionals (TLS alternatius) no hi són.
-dontwarn org.bouncycastle.jsse.**
-dontwarn org.conscrypt.**
-dontwarn org.openjsse.**
