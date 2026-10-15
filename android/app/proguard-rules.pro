# R8 rules of the release build. The libraries ship their own consumer rules;
# these cover what R8 cannot see.

# Stack traces in Enviar registro stay readable with the mapping file.
-keepattributes SourceFile,LineNumberTable

# youtubedl-android unpacks Python and ffmpeg with commons-compress, whose
# ExtraFieldUtils creates each ZipExtraField by reflection (Class.newInstance).
-keep class * implements org.apache.commons.compress.archivers.zip.ZipExtraField { <init>(); }
