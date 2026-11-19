import 'dart:convert';
import 'dart:async';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import 'package:flutter/foundation.dart';
import "package:flutter/services.dart";
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:firebase_analytics/firebase_analytics.dart';

import 'firebase_options.dart';

const _cfId = String.fromEnvironment('CF_ID');
const _cfSecret = String.fromEnvironment('CF_SECRET');
const _defaultBaseUrl = String.fromEnvironment('BASE_URL');

const _authHeaders = {
  'CF-Access-Client-Id': _cfId,
  'CF-Access-Client-Secret': _cfSecret,
};
// Текущий режим темы. За ним следят MaterialApp и панель профиля.
final themeModeNotifier = ValueNotifier<ThemeMode>(ThemeMode.system);

Future<void> _setThemeMode(ThemeMode mode) async {
  themeModeNotifier.value = mode; // экран перекрасится сразу
  final prefs = await SharedPreferences.getInstance();
  await prefs.setString('theme_mode', mode.name); // и запомним на потом
}

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp(options: DefaultFirebaseOptions.currentPlatform);
  final prefs = await SharedPreferences.getInstance();
  final saved = prefs.getString('theme_mode');
  themeModeNotifier.value = ThemeMode.values.firstWhere(
    (m) => m.name == saved,
    orElse: () => ThemeMode.system,
  );
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  // This widget is the root of your application.
  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<ThemeMode>(
      valueListenable: themeModeNotifier,
      builder: (context, mode, _) => MaterialApp(
        title: 'My service',
        theme: ThemeData(colorScheme: .fromSeed(seedColor: Colors.blue)),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: Colors.blue,
            brightness: Brightness.dark,
          ),
        ),
        themeMode: mode,
        home: MyHomePage(title: 'Mini Imgur'),
      ),
    );
  }
}

// Экран входа. Вынесен в отдельный виджет, чтобы его можно было проверить тестом без Firebase.
class LoginScreen extends StatelessWidget {
  const LoginScreen({super.key, required this.title, required this.onSignIn});

  final String title;
  final VoidCallback onSignIn;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: Center(
        child: FilledButton.icon(
          icon: const Icon(Icons.login),
          label: const Text('Войти через Google'),
          onPressed: onSignIn,
        ),
      ),
    );
  }
}

// Переключатель темы: Авто / Светлая / Тёмная.
class ThemeModeSelector extends StatelessWidget {
  const ThemeModeSelector({super.key});

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<ThemeMode>(
      valueListenable: themeModeNotifier,
      builder: (_, mode, _) => SegmentedButton<ThemeMode>(
        segments: const [
          ButtonSegment(
            value: ThemeMode.system,
            icon: Icon(Icons.brightness_auto),
            label: Text('Авто'),
          ),
          ButtonSegment(
            value: ThemeMode.light,
            icon: Icon(Icons.light_mode),
            label: Text('Светлая'),
          ),
          ButtonSegment(
            value: ThemeMode.dark,
            icon: Icon(Icons.dark_mode),
            label: Text('Тёмная'),
          ),
        ],
        selected: {mode},
        onSelectionChanged: (s) => _setThemeMode(s.first),
      ),
    );
  }
}

class MyHomePage extends StatefulWidget {
  const MyHomePage({super.key, required this.title});

  final String title;

  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

class ImageViewer extends StatelessWidget {
  final String url;
  final Map<String, String> headers;
  const ImageViewer({super.key, required this.url, required this.headers});

  @override
  Widget build(BuildContext context) {
    return InteractiveViewer(
      child: Image.network(
        url,
        headers: headers,
        fit: BoxFit.contain,
        loadingBuilder: (context, child, progress) {
          if (progress == null) return child;
          return const Center(child: CircularProgressIndicator());
        },
        errorBuilder: (context, error, stack) => const Center(
          child: Text('Не удалось показать оригинал (возможно, HEIC)'),
        ),
      ),
    );
  }
}

class _MyHomePageState extends State<MyHomePage> {
  List<dynamic> _images = [];
  User? _user;
  String? _token;
  StreamSubscription<User?>? _authSub;
  WebSocketChannel? _channel;
  Timer? _reconnectTimer;
  String _baseUrl = '';

  Map<String, String> get _headers => {
    if (!kIsWeb && _baseUrl.startsWith('https')) ..._authHeaders,
    if (_token != null) 'Authorization': 'Bearer $_token',
  };
  Future<void> _loadSettings() async {
    final prefs = await SharedPreferences.getInstance();
    final saved = (kIsWeb && kReleaseMode)
        ? Uri
              .base
              .origin // сайт открыт с того же домена, что и API
        : (prefs.getString('base_url') ?? _defaultBaseUrl);
    if (!mounted) return;
    setState(() => _baseUrl = saved);
    await _askNetworkPermission();
  }

  Future<void> _askNetworkPermission() async {
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      await Permission.accessLocalNetwork.request();
    }
  }

  Future<void> _uploadImage() async {
    if (_baseUrl.isEmpty) return;
    final file = await FilePicker.pickFile(type: FileType.image);
    if (file == null) return; // отменил выбор

    final bytes = await file.readAsBytes();
    final request = http.MultipartRequest(
      'POST',
      Uri.parse('$_baseUrl/images'),
    );
    request.headers.addAll(_headers);
    request.files.add(
      http.MultipartFile.fromBytes('file', bytes, filename: file.name),
    );

    try {
      final response = await request.send();
      if (response.statusCode == 200 || response.statusCode == 201) {
        FirebaseAnalytics.instance.logEvent(
          name: 'image_upload',
          parameters: {'format': file.extension ?? 'unknown'},
        );
        _loadImages();
      } else {
        debugPrint('upload failed: ${response.statusCode}');
      }
    } catch (e) {
      debugPrint('upload error: $e');
    }
  }

  @override
  void initState() {
    super.initState();
    _authSub = FirebaseAuth.instance.idTokenChanges().listen((u) async {
      final token = await u?.getIdToken();
      if (!mounted) return;
      setState(() {
        _user = u;
        _token = token;
      });
      if (token == null) {
        _disconnectWs();
        setState(() => _images = []);
      } else {
        _registerUser();
        _connectWs();
        _loadImages();
      }
    });
    _loadSettings().then((_) {
      _registerUser();
      _loadImages();
      _connectWs();
    });
  }

  @override
  void dispose() {
    _disconnectWs();
    _authSub?.cancel();
    super.dispose();
  }

  void _connectWs() {
    if (_channel != null || _token == null || _baseUrl.isEmpty) return;
    final url = '${_baseUrl.replaceFirst('http', 'ws')}/ws?token=$_token';
    final channel = WebSocketChannel.connect(Uri.parse(url));
    _channel = channel;
    channel.stream.listen(
      (_) => _loadImages(),
      onDone: () {
        if (_channel == channel) _scheduleReconnect();
      },
      onError: (_) {
        if (_channel == channel) _scheduleReconnect();
      },
    );
    _loadImages(); // подхватить то, что могли пропустить
  }

  void _scheduleReconnect() {
    _channel = null;
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 5), _connectWs);
  }

  void _disconnectWs() {
    final channel = _channel;
    _channel = null;
    _reconnectTimer?.cancel();
    channel?.sink.close();
  }

  Future<void> _registerUser() async {
    if (_baseUrl.isEmpty || _token == null) return;
    try {
      await http.get(Uri.parse('$_baseUrl/me'), headers: _headers);
    } catch (e) {
      debugPrint('me error: $e');
    }
  }

  Future<void> _loadImages() async {
    if (_baseUrl.isEmpty) return;
    if (_token == null) return; // не вошли: нечего спрашивать
    try {
      final response = await http.get(
        Uri.parse('$_baseUrl/images'),
        headers: _headers,
      );
      if (response.statusCode != 200) {
        debugPrint('load failed: ${response.statusCode}');
        return;
      }
      final data = jsonDecode(response.body);
      if (!mounted) return;
      setState(() {
        _images = data ?? [];
      });
    } catch (e) {
      debugPrint('load error: $e');
    }
  }

  bool _googleReady = false;

  Future<void> _signIn() async {
    final provider = GoogleAuthProvider();
    provider.setCustomParameters({'prompt': 'select_account'});
    try {
      if (kIsWeb) {
        await FirebaseAuth.instance.signInWithPopup(provider);
      } else {
        if (!_googleReady) {
          await GoogleSignIn.instance.initialize();
          _googleReady = true;
        }
        final account = await GoogleSignIn.instance.authenticate();
        final idToken = account.authentication.idToken;
        await FirebaseAuth.instance.signInWithCredential(
          GoogleAuthProvider.credential(idToken: idToken),
        );
      }
      await FirebaseAnalytics.instance.logLogin(loginMethod: 'google');
    } catch (e) {
      debugPrint('sign in error: $e');
    }
  }

  void _openImage(dynamic img) {
    final url = '$_baseUrl/images/${img['id']}/view';
    final viewer = ImageViewer(url: url, headers: _headers);
    final wide = MediaQuery.of(context).size.width > 600;

    if (wide) {
      showDialog(
        context: context,
        builder: (ctx) => Dialog(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 900, maxHeight: 700),
            child: Column(
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    IconButton(
                      icon: const Icon(Icons.download),
                      tooltip: 'Скачать оригинал',
                      onPressed: () => _downloadOriginal(img),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close),
                      onPressed: () => Navigator.pop(ctx),
                    ),
                  ],
                ),
                Expanded(child: viewer),
              ],
            ),
          ),
        ),
      );
    } else {
      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (_) => Scaffold(
            appBar: AppBar(
              actions: [
                IconButton(
                  icon: const Icon(Icons.download),
                  onPressed: () => _downloadOriginal(img),
                ),
              ],
            ),
            body: viewer,
          ),
        ),
      );
    }
  }

  Widget _buildTile(dynamic img) {
    final status = img['status'];

    if (status == 'done') {
      return GestureDetector(
        onTap: () => _openImage(img),
        child: Image.network(
          '$_baseUrl/images/${img['id']}/thumbnail',
          headers: _headers,
          fit: BoxFit.cover,
          loadingBuilder: (context, child, progress) {
            if (progress == null) return child;
            return const Center(
              child: CircularProgressIndicator(strokeWidth: 2),
            );
          },
          errorBuilder: (context, error, stack) =>
              const Center(child: Icon(Icons.broken_image)),
        ),
      );
    }

    if (status == 'failed') {
      return Container(
        color: Theme.of(context).colorScheme.errorContainer,
        child: Center(
          child: Icon(
            Icons.error_outline,
            color: Theme.of(context).colorScheme.error,
          ),
        ),
      );
    }

    // pending: worker ещё делает превью
    return Container(
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: const Center(child: CircularProgressIndicator(strokeWidth: 2)),
    );
  }

  Future<void> _downloadOriginal(dynamic img) async {
    final url = Uri.parse('$_baseUrl/images/${img['id']}/original');
    await launchUrl(url, mode: LaunchMode.externalApplication);
  }

  void _showProfile() {
    final user = _user;
    if (user == null) return;
    showModalBottomSheet(
      context: context,
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              CircleAvatar(
                radius: 36,
                backgroundImage: user.photoURL != null
                    ? NetworkImage(user.photoURL!)
                    : null,
                child: user.photoURL == null
                    ? const Icon(Icons.person, size: 36)
                    : null,
              ),
              const SizedBox(height: 12),
              Text(
                user.displayName ?? 'Без имени',
                style: Theme.of(ctx).textTheme.titleLarge,
              ),
              Text(user.email ?? ''),
              const SizedBox(height: 8),
              Text('Картинок: ${_images.length}'),
              const SizedBox(height: 16),
              const ThemeModeSelector(),
              const SizedBox(height: 16),
              FilledButton.icon(
                icon: const Icon(Icons.logout),
                label: const Text('Выйти'),
                onPressed: () {
                  Navigator.pop(ctx);
                  FirebaseAuth.instance.signOut();
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _editServer() async {
    final controller = TextEditingController(text: _baseUrl);
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Server address'),
        content: TextField(
          controller: controller,
          decoration: const InputDecoration(
            hintText: 'https://your-server.example',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: const Text('Save'),
          ),
        ],
      ),
    );
    if (result == null) return; // нажал Cancel
    final url = result.replaceAll(RegExp(r'/+$'), '');
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('base_url', url);
    setState(() => _baseUrl = url);
    _disconnectWs();
    _connectWs();
    _loadImages();
  }

  @override
  Widget build(BuildContext context) {
    if (_user == null) {
      return LoginScreen(title: widget.title, onSignIn: _signIn);
    }
    return Scaffold(
      appBar: AppBar(
        backgroundColor: Theme.of(context).colorScheme.inversePrimary,

        title: Text(widget.title),
        actions: [
          IconButton(
            tooltip: 'Профиль',
            icon: CircleAvatar(
              radius: 14,
              backgroundImage: _user?.photoURL != null
                  ? NetworkImage(_user!.photoURL!)
                  : null,
              child: _user?.photoURL == null
                  ? const Icon(Icons.person, size: 16)
                  : null,
            ),
            onPressed: _showProfile,
          ),
          IconButton(icon: const Icon(Icons.settings), onPressed: _editServer),
          if (kDebugMode)
            IconButton(
              icon: const Icon(Icons.key),
              tooltip: 'Copy token',
              onPressed: () =>
                  Clipboard.setData(ClipboardData(text: _token ?? '')),
            ),
        ],
      ),
      body: _images.isEmpty
          ? Center(
              child: Text(
                'Unfortunately you have no pics at the moment',
                style: TextStyle(
                  fontSize: 20,
                  color: Theme.of(context).colorScheme.primary,
                ),
              ),
            )
          : GridView.builder(
              padding: const EdgeInsets.all(8),
              itemCount: _images.length,
              itemBuilder: (context, i) {
                return ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: _buildTile(_images[i]),
                );
              },
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 3,
                mainAxisSpacing: 8,
                crossAxisSpacing: 8,
              ),
            ),
      floatingActionButton: FloatingActionButton(
        tooltip: 'Upload', // used by assistive technologies
        onPressed: _uploadImage,
        child: Icon(Icons.add),
      ),
    );
  }
}
