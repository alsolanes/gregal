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
